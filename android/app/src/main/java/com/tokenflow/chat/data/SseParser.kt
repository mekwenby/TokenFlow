package com.tokenflow.chat.data

import okio.BufferedSource

data class RawSseEvent(val event: String, val data: String)

class SseParser {
    suspend fun read(source: BufferedSource, onEvent: suspend (RawSseEvent) -> Unit) {
        var event = ""
        val data = mutableListOf<String>()

        suspend fun dispatch() {
            if (data.isEmpty()) {
                event = ""
                return
            }
            onEvent(RawSseEvent(event, data.joinToString("\n")))
            event = ""
            data.clear()
        }

        while (true) {
            val line = source.readUtf8Line() ?: break
            if (line.isEmpty()) {
                dispatch()
                continue
            }
            if (line.startsWith(':')) continue
            val separator = line.indexOf(':')
            val field = if (separator >= 0) line.substring(0, separator) else line
            val value = if (separator >= 0) line.substring(separator + 1).removePrefix(" ") else ""
            when (field) {
                "event" -> event = value
                "data" -> data += value
            }
        }
        dispatch()
    }
}
