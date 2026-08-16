package com.tokenflow.chat.ui

import org.junit.Assert.assertEquals
import org.junit.Test

class ChatFormattingTest {
    @Test
    fun tokenCountUsesCompactKNotation() {
        assertEquals("0", formatTokenCount(-1))
        assertEquals("999", formatTokenCount(999))
        assertEquals("1K", formatTokenCount(1_000))
        assertEquals("1K", formatTokenCount(1_001))
        assertEquals("1.2K", formatTokenCount(1_200))
        assertEquals("12.4K", formatTokenCount(12_400))
        assertEquals("1000K", formatTokenCount(999_999))
    }
}
