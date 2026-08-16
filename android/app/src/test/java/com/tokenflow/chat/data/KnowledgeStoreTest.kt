package com.tokenflow.chat.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class KnowledgeStoreTest {
    @Test
    fun chunksLongTextWithOverlapAndBoundedSize() {
        val text = (1..400).joinToString("\n\n") { "Paragraph $it contains enough searchable content." }
        val chunks = KnowledgeStore.chunk(text)

        assertTrue(chunks.size > 1)
        assertTrue(chunks.all { it.length <= 1_200 })
        assertTrue(chunks.zipWithNext().all { (left, right) ->
            left.takeLast(80).split(' ').any { it.length > 4 && right.contains(it) }
        })
    }

    @Test
    fun searchTextIncludesLatinTermsAndChineseBigrams() {
        val searchable = KnowledgeStore.searchable("Compose Room 数据库迁移与本地知识检索")

        assertTrue(searchable.contains("compose"))
        assertTrue(searchable.contains("room"))
        assertTrue(searchable.contains("数据"))
        assertTrue(searchable.contains("知识"))
        assertEquals(searchable.split(' ').distinct().size, searchable.split(' ').size)
    }
}
