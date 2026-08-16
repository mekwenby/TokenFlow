package com.tokenflow.chat.data

import android.content.Context

class ChatDisplayPreferences(context: Context) {
    private val preferences = context.applicationContext.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE)

    fun readFontScale(): Float = preferences
        .getFloat(CHAT_FONT_SCALE, DEFAULT_FONT_SCALE)
        .coerceIn(MIN_FONT_SCALE, MAX_FONT_SCALE)

    fun writeFontScale(value: Float) {
        preferences.edit().putFloat(CHAT_FONT_SCALE, value.coerceIn(MIN_FONT_SCALE, MAX_FONT_SCALE)).apply()
    }

    companion object {
        const val MIN_FONT_SCALE = 0.8f
        const val MAX_FONT_SCALE = 1.4f
        const val DEFAULT_FONT_SCALE = 1f

        private const val PREFERENCES = "tokenflow_display"
        private const val CHAT_FONT_SCALE = "chat_font_scale"
    }
}
