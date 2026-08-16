package com.tokenflow.chat.data

import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ExaClientTest {
    private lateinit var server: MockWebServer

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun sendsFixedSearchParametersAndTruncatesToolOutput() = runTest {
        val longHighlight = "x".repeat(25_000)
        server.enqueue(MockResponse().setHeader("Content-Type", "application/json").setBody(
            "{\"results\":[{\"title\":\"Example\",\"url\":\"https://example.com\",\"highlights\":[\"$longHighlight\"]}]}",
        ))
        val client = ExaClient(endpointUrl = server.url("/search").toString())

        val result = client.search("exa-key", "latest docs")
        val request = server.takeRequest()
        val body = DirectApiTransport.defaultJson.parseToJsonElement(request.body.readUtf8()).jsonObject

        assertEquals("Bearer exa-key", request.getHeader("Authorization"))
        assertEquals("auto", body["type"]?.jsonPrimitive?.content)
        assertEquals("5", body["numResults"]?.jsonPrimitive?.content)
        assertTrue(body["contents"].toString().contains("highlights"))
        assertTrue(result.length <= ExaClient.MAX_TOOL_CHARS + "\n[truncated]".length)
        assertTrue(result.endsWith("[truncated]"))
    }
}
