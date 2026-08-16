package com.tokenflow.chat.ui

import androidx.test.platform.app.InstrumentationRegistry
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference
import kotlin.math.PI
import kotlin.math.sin
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SpeechPlaybackControllerTest {
    @Test
    fun validWaveReachesPlayingAndCanPause() {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val context = instrumentation.targetContext
        val wave = context.cacheDir.resolve("speech-player-test.wav")
        wave.writeBytes(testWave())
        val playing = CountDownLatch(1)
        val latest = AtomicReference(SpeechPlaybackState())
        lateinit var controller: SpeechPlaybackController

        instrumentation.runOnMainSync {
            controller = SpeechPlaybackController(
                context = context,
                onStateChanged = { state ->
                    latest.set(state)
                    if (state.phase == SpeechPlaybackPhase.PLAYING) playing.countDown()
                },
                onError = { error("Unexpected playback error for $it") },
            )
            controller.play("message", wave.absolutePath)
        }

        assertTrue("Audio never reached the playing state", playing.await(5, TimeUnit.SECONDS))
        instrumentation.runOnMainSync { controller.playOrToggle("message", wave.absolutePath) }
        instrumentation.waitForIdleSync()
        assertEquals(SpeechPlaybackPhase.PAUSED, latest.get().phase)

        instrumentation.runOnMainSync { controller.release() }
        wave.delete()
    }

    private fun testWave(): ByteArray {
        val sampleRate = 24_000
        val sampleCount = sampleRate / 2
        val dataSize = sampleCount * 2
        return ByteBuffer.allocate(44 + dataSize).order(ByteOrder.LITTLE_ENDIAN).apply {
            put("RIFF".encodeToByteArray())
            putInt(36 + dataSize)
            put("WAVEfmt ".encodeToByteArray())
            putInt(16)
            putShort(1.toShort())
            putShort(1.toShort())
            putInt(sampleRate)
            putInt(sampleRate * 2)
            putShort(2.toShort())
            putShort(16.toShort())
            put("data".encodeToByteArray())
            putInt(dataSize)
            repeat(sampleCount) { index ->
                putShort((sin(2.0 * PI * 440.0 * index / sampleRate) * 4_000).toInt().toShort())
            }
        }.array()
    }
}
