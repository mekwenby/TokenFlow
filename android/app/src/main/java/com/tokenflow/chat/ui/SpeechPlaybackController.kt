package com.tokenflow.chat.ui

import android.content.Context
import android.net.Uri
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import java.io.File

internal enum class SpeechPlaybackPhase {
    IDLE,
    PREPARING,
    PLAYING,
    PAUSED,
    ENDED,
    ERROR,
}

internal data class SpeechPlaybackState(
    val messageId: String? = null,
    val phase: SpeechPlaybackPhase = SpeechPlaybackPhase.IDLE,
    val durationMs: Long = 0,
)

/** Owns the single speech player used by the chat screen. */
internal class SpeechPlaybackController(
    context: Context,
    private val onStateChanged: (SpeechPlaybackState) -> Unit,
    private val onError: (String) -> Unit,
) {
    private val appContext = context.applicationContext
    private var currentMessageId: String? = null
    private var released = false
    private var player: ExoPlayer? = null

    private val listener = object : Player.Listener {
        override fun onPlaybackStateChanged(playbackState: Int) {
            val id = currentMessageId ?: return
            val activePlayer = player ?: return
            when (playbackState) {
                Player.STATE_BUFFERING -> emit(id, SpeechPlaybackPhase.PREPARING)
                Player.STATE_READY -> emit(
                    id,
                    if (activePlayer.isPlaying) SpeechPlaybackPhase.PLAYING else SpeechPlaybackPhase.PAUSED,
                )
                Player.STATE_ENDED -> {
                    activePlayer.playWhenReady = false
                    emit(id, SpeechPlaybackPhase.ENDED)
                    activePlayer.stop()
                }
                Player.STATE_IDLE -> Unit
            }
        }

        override fun onIsPlayingChanged(isPlaying: Boolean) {
            val id = currentMessageId ?: return
            if (player?.playbackState == Player.STATE_READY) {
                emit(id, if (isPlaying) SpeechPlaybackPhase.PLAYING else SpeechPlaybackPhase.PAUSED)
            }
        }

        override fun onPlayerError(error: PlaybackException) {
            val id = currentMessageId ?: return
            emit(id, SpeechPlaybackPhase.ERROR)
            onError(id)
        }
    }

    fun playOrToggle(messageId: String, filePath: String) {
        if (released) return
        val activePlayer = player
        if (currentMessageId != messageId || activePlayer == null) {
            play(messageId, filePath)
            return
        }
        when (activePlayer.playbackState) {
            Player.STATE_BUFFERING -> Unit
            Player.STATE_ENDED -> {
                activePlayer.seekToDefaultPosition()
                activePlayer.play()
            }
            Player.STATE_READY -> if (activePlayer.isPlaying) activePlayer.pause() else activePlayer.play()
            else -> play(messageId, filePath)
        }
    }

    fun play(messageId: String, filePath: String) {
        if (released) return
        val file = File(filePath)
        if (!file.isFile || file.length() <= 44) {
            emit(messageId, SpeechPlaybackPhase.ERROR)
            onError(messageId)
            return
        }
        val activePlayer = player ?: createPlayer().also { player = it }
        currentMessageId = messageId
        emit(messageId, SpeechPlaybackPhase.PREPARING)
        activePlayer.setMediaItem(MediaItem.fromUri(Uri.fromFile(file)))
        activePlayer.prepare()
        activePlayer.playWhenReady = true
    }

    fun stop(messageId: String) {
        if (currentMessageId != messageId || released) return
        player?.stop()
        player?.clearMediaItems()
        currentMessageId = null
        onStateChanged(SpeechPlaybackState())
    }

    fun release() {
        if (released) return
        released = true
        currentMessageId = null
        player?.release()
        player = null
        onStateChanged(SpeechPlaybackState())
    }

    private fun createPlayer(): ExoPlayer = ExoPlayer.Builder(appContext).build().apply {
        setAudioAttributes(
            AudioAttributes.Builder()
                .setUsage(C.USAGE_MEDIA)
                .setContentType(C.AUDIO_CONTENT_TYPE_SPEECH)
                .build(),
            true,
        )
        setHandleAudioBecomingNoisy(true)
        volume = 1f
        addListener(listener)
    }

    private fun emit(messageId: String, phase: SpeechPlaybackPhase) {
        val duration = player?.duration?.takeIf { it != C.TIME_UNSET && it > 0 } ?: 0
        onStateChanged(SpeechPlaybackState(messageId, phase, duration))
    }
}
