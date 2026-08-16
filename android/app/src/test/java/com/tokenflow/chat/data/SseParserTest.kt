package com.tokenflow.chat.data

import kotlinx.coroutines.test.runTest
import okio.Buffer
import org.junit.Assert.assertEquals
import org.junit.Test

class SseParserTest {
    @Test
    fun parsesNamedMultilineEventsAndFinalEventWithoutBlankLine() = runTest {
        val source = Buffer().writeUtf8(
            ": keepalive\n" +
                "event: response.output_text.delta\n" +
                "data: {\"delta\":\"hello\"}\n" +
                "data: second-line\n\n" +
                "event: response.completed\n" +
                "data: {\"type\":\"response.completed\"}",
        )
        val events = mutableListOf<RawSseEvent>()

        SseParser().read(source) { events += it }

        assertEquals(2, events.size)
        assertEquals("response.output_text.delta", events[0].event)
        assertEquals("{\"delta\":\"hello\"}\nsecond-line", events[0].data)
        assertEquals("response.completed", events[1].event)
    }
}
