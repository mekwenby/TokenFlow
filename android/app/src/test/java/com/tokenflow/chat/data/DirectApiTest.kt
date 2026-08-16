package com.tokenflow.chat.data

import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.util.Base64
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.RecordedRequest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger

class DirectApiTest {
    private lateinit var server: MockWebServer
    private lateinit var gateway: ModelGateway

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        gateway = ModelGateway()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun chatCompletionsUsesBearerAndParsesTextToolsAndUsage() = runTest {
        server.enqueue(sse(
            "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\",\"reasoning_content\":\"Think\",\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"function\":{\"name\":\"read_url\",\"arguments\":\"{}\"}}]}}]}\n\n" +
                "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":2}}\n\n" +
                "data: [DONE]\n\n",
        ))

        val events = gateway.stream(request(ProviderProtocol.OPENAI_CHAT_COMPLETIONS)).toList()
        val recorded = server.takeRequest()

        assertEquals("/v1/chat/completions", recorded.path)
        assertEquals("Bearer secret", recorded.getHeader("Authorization"))
        assertEquals("request-1", recorded.getHeader("X-Client-Request-Id"))
        assertTrue(recorded.body.readUtf8().contains("\"reasoning_effort\":\"medium\""))
        assertTrue(events.any { it == ModelStreamEvent.TextDelta("Hello") })
        assertTrue(events.any { it == ModelStreamEvent.ThinkingDelta("Think") })
        assertTrue(events.any { it is ModelStreamEvent.ToolCallDelta && it.name == "read_url" })
        assertTrue(events.any { it is ModelStreamEvent.TokenUsage && it.usage.totalTokens == 6L })
    }

    @Test
    fun openAIModelListUsesApiRootAndRequiresExplicitSelectionByCaller() = runTest {
        server.enqueue(json("{\"data\":[{\"id\":\"model-b\"},{\"id\":\"model-a\"},{\"id\":\"model-a\"}]}"))
        val provider = provider(ProviderProtocol.OPENAI_RESPONSES)

        val models = gateway.listModels(provider, "secret")
        val recorded = server.takeRequest()

        assertEquals("/v1/models", recorded.path)
        assertEquals("Bearer secret", recorded.getHeader("Authorization"))
        assertEquals(listOf("model-a", "model-b"), models.map { it.id })
    }

    @Test
    fun responsesUsesNamedEventsAndNormalizesFunctionArguments() = runTest {
        server.enqueue(sse(
            "event: response.reasoning_summary_text.delta\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"summary\"}\n\n" +
                "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":1,\"item\":{\"type\":\"function_call\",\"id\":\"fc-2\",\"call_id\":\"call-2\",\"name\":\"web_search\",\"arguments\":\"\"}}\n\n" +
                "event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"output_index\":1,\"call_id\":\"call-2\",\"delta\":\"{\\\"query\\\":\\\"news\\\"}\"}\n\n" +
                "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Answer\"}\n\n" +
                "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":8,\"output_tokens\":3}}}\n\n",
        ))

        val events = gateway.stream(request(ProviderProtocol.OPENAI_RESPONSES)).toList()
        val recorded = server.takeRequest()

        assertEquals("/v1/responses", recorded.path)
        assertTrue(events.any { it == ModelStreamEvent.ThinkingDelta("summary") })
        assertTrue(events.any { it is ModelStreamEvent.ToolCallDelta && it.id == "call-2" && it.name == "web_search" })
        assertTrue(events.any { it is ModelStreamEvent.ToolCallDelta && it.arguments.contains("news") })
        assertTrue(events.any { it == ModelStreamEvent.TextDelta("Answer") })
        assertTrue(events.any { it is ModelStreamEvent.TokenUsage && it.usage.totalTokens == 11L })
    }

    @Test
    fun anthropicListsPagesAndUsesAnthropicHeaders() = runTest {
        server.enqueue(json("{\"data\":[{\"id\":\"claude-a\",\"display_name\":\"Claude A\"}],\"has_more\":true,\"last_id\":\"claude-a\"}"))
        server.enqueue(json("{\"data\":[{\"id\":\"claude-b\",\"display_name\":\"Claude B\"}],\"has_more\":false}"))
        val provider = provider(ProviderProtocol.ANTHROPIC_MESSAGES)

        val models = gateway.listModels(provider, "anthropic-secret")
        val first = server.takeRequest()
        val second = server.takeRequest()

        assertEquals(listOf("claude-a", "claude-b"), models.map { it.id })
        assertEquals("anthropic-secret", first.getHeader("x-api-key"))
        assertEquals(DirectApiTransport.ANTHROPIC_VERSION, first.getHeader("anthropic-version"))
        assertEquals("/v1/models?after_id=claude-a", second.path)
    }

    @Test
    fun adaptersEncodeProtocolSpecificImageParts() = runTest {
        val image = CanonicalContentPart.Image("image/png", Base64.getEncoder().encodeToString(byteArrayOf(1, 2, 3)))
        ProviderProtocol.entries.forEach { protocol ->
            server.enqueue(sse(when (protocol) {
                ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> "data: [DONE]\n\n"
                ProviderProtocol.OPENAI_RESPONSES -> "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n"
                ProviderProtocol.ANTHROPIC_MESSAGES -> "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
            }))
            gateway.stream(request(protocol).copy(messages = listOf(CanonicalMessage(
                role = "user",
                parts = listOf(CanonicalContentPart.Text("look"), image),
            )))).toList()
            val body = server.takeRequest().body.readUtf8()
            when (protocol) {
                ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> assertTrue(body.contains("image_url"))
                ProviderProtocol.OPENAI_RESPONSES -> assertTrue(body.contains("input_image"))
                ProviderProtocol.ANTHROPIC_MESSAGES -> {
                    assertTrue(body.contains("\"type\":\"image\""))
                    assertTrue(body.contains("\"media_type\":\"image/png\""))
                }
            }
        }
    }

    @Test
    fun adaptersEncodePlainSecondTurnAndDropEmptyAssistantMessages() = runTest {
        ProviderProtocol.entries.forEach { protocol ->
            server.enqueue(sse(when (protocol) {
                ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> "data: [DONE]\n\n"
                ProviderProtocol.OPENAI_RESPONSES ->
                    "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n"
                ProviderProtocol.ANTHROPIC_MESSAGES ->
                    "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
            }))
            gateway.stream(request(protocol).copy(messages = listOf(
                CanonicalMessage("user", "first"),
                CanonicalMessage("assistant", "answer"),
                CanonicalMessage("assistant"),
                CanonicalMessage("user", "second"),
            ))).toList()

            val body = JSON.parseToJsonElement(server.takeRequest().body.readUtf8()).jsonObject
            when (protocol) {
                ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> {
                    val messages = body.getValue("messages").jsonArray.map { it.jsonObject }
                    assertEquals(listOf("system", "user", "assistant", "user"), messages.map(::role))
                    assertEquals("answer", messages[2].getValue("content").jsonPrimitive.content)
                }
                ProviderProtocol.OPENAI_RESPONSES -> {
                    val input = body.getValue("input").jsonArray.map { it.jsonObject }
                    assertEquals(listOf("user", "assistant", "user"), input.map(::role))
                    assertEquals("answer", input[1].getValue("content").jsonPrimitive.content)
                }
                ProviderProtocol.ANTHROPIC_MESSAGES -> {
                    val messages = body.getValue("messages").jsonArray.map { it.jsonObject }
                    assertEquals(listOf("user", "assistant", "user"), messages.map(::role))
                    assertEquals("answer", messages[1].getValue("content").jsonArray.single()
                        .jsonObject.getValue("text").jsonPrimitive.content)
                }
            }
        }
    }

    @Test
    fun responsesSecondTurnSurvivesGatewayThatRejectsAssistantInputParts() = runTest {
        val requestCount = AtomicInteger()
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                val turn = requestCount.incrementAndGet()
                val body = JSON.parseToJsonElement(request.body.readUtf8()).jsonObject
                if (turn > 1) {
                    val invalidAssistant = body.getValue("input").jsonArray
                        .map { it.jsonObject }
                        .filter { it["role"]?.jsonPrimitive?.content == "assistant" }
                        .any { it["content"] is JsonArray }
                    if (invalidAssistant) return MockResponse()
                        .setResponseCode(502)
                        .setBody("{\"error\":{\"message\":\"Upstream request failed\"}}")
                }
                return sse(
                    "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"turn-$turn\"}\n\n" +
                        "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n",
                )
            }
        }
        val engine = DirectChatEngine(gateway, NoTools)
        val initial = request(ProviderProtocol.OPENAI_RESPONSES).copy(
            thinkingEffort = "off",
            messages = listOf(CanonicalMessage("user", "first")),
        )

        val first = engine.run(initial, false, false, 0).toList().last() as EngineEvent.Done
        val second = engine.run(initial.copy(messages = listOf(
            CanonicalMessage("user", "first"),
            CanonicalMessage("assistant", first.content),
            CanonicalMessage("user", "second"),
        )), false, false, 0).toList().last() as EngineEvent.Done

        assertEquals("turn-1", first.content)
        assertEquals("turn-2", second.content)
        assertEquals(2, requestCount.get())
    }

    @Test
    fun responsesUsesPartsOnlyForImageUserMessages() = runTest {
        server.enqueue(sse("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n"))
        val image = CanonicalContentPart.Image("image/png", Base64.getEncoder().encodeToString(byteArrayOf(1, 2, 3)))

        gateway.stream(request(ProviderProtocol.OPENAI_RESPONSES).copy(messages = listOf(
            CanonicalMessage("user", "first"),
            CanonicalMessage("assistant", "answer"),
            CanonicalMessage("user", parts = listOf(CanonicalContentPart.Text("look"), image)),
        ))).toList()

        val input = requestBody().getValue("input").jsonArray.map { it.jsonObject }
        assertEquals("first", input[0].getValue("content").jsonPrimitive.content)
        assertEquals("answer", input[1].getValue("content").jsonPrimitive.content)
        assertEquals(listOf("input_text", "input_image"), input[2].getValue("content").jsonArray.map {
            it.jsonObject.getValue("type").jsonPrimitive.content
        })
    }

    @Test
    fun responsesReplaysOutputItemsAndPairsFunctionOutput() = runTest {
        server.enqueue(sse(
            "event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"id\":\"rs-1\",\"type\":\"reasoning\",\"encrypted_content\":\"sealed\",\"summary\":[]}}\n\n" +
                "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":1,\"item\":{\"id\":\"fc-1\",\"type\":\"function_call\",\"call_id\":\"call-1\",\"name\":\"read_url\",\"arguments\":\"\"}}\n\n" +
                "event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"output_index\":1,\"item_id\":\"fc-1\",\"delta\":\"{\\\"url\\\":\\\"https://example.com\\\"}\"}\n\n" +
                "event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"output_index\":1,\"item\":{\"id\":\"fc-1\",\"type\":\"function_call\",\"call_id\":\"call-1\",\"name\":\"read_url\",\"arguments\":\"{\\\"url\\\":\\\"https://example.com\\\"}\"}}\n\n" +
                "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n",
        ))
        server.enqueue(sse(
            "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"final\"}\n\n" +
                "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n",
        ))
        val engine = DirectChatEngine(gateway, FakeTools)

        val events = engine.run(request(ProviderProtocol.OPENAI_RESPONSES), false, true, 7).toList()
        server.takeRequest()
        val input = requestBody().getValue("input").jsonArray.map { it.jsonObject }

        assertEquals(listOf("reasoning", "function_call", "function_call_output"), input.drop(1).map {
            it.getValue("type").jsonPrimitive.content
        })
        assertEquals("sealed", input[1].getValue("encrypted_content").jsonPrimitive.content)
        assertEquals("call-1", input[2].getValue("call_id").jsonPrimitive.content)
        assertEquals("call-1", input[3].getValue("call_id").jsonPrimitive.content)
        assertEquals("page contents", input[3].getValue("output").jsonPrimitive.content)
        assertEquals("final", (events.last() as EngineEvent.Done).content)
    }

    @Test
    fun anthropicReplaysThinkingSignatureAndValidToolInput() = runTest {
        server.enqueue(sse(
            "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\",\"signature\":\"\"}}\n\n" +
                "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"consider\"}}\n\n" +
                "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"signed\"}}\n\n" +
                "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
                "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool-1\",\"name\":\"read_url\",\"input\":{}}}\n\n" +
                "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"url\\\":\\\"https://example.com\\\"}\"}}\n\n" +
                "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n" +
                "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
        ))
        server.enqueue(sse(
            "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"final\"}}\n\n" +
                "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
        ))
        val engine = DirectChatEngine(gateway, FakeTools)

        val events = engine.run(request(ProviderProtocol.ANTHROPIC_MESSAGES), false, true, 7).toList()
        server.takeRequest()
        val messages = requestBody().getValue("messages").jsonArray.map { it.jsonObject }
        val assistantBlocks = messages[1].getValue("content").jsonArray.map { it.jsonObject }
        val resultBlocks = messages[2].getValue("content").jsonArray.map { it.jsonObject }

        assertEquals(listOf("user", "assistant", "user"), messages.map(::role))
        assertEquals(listOf("thinking", "tool_use"), assistantBlocks.map(::type))
        assertEquals("consider", assistantBlocks[0].getValue("thinking").jsonPrimitive.content)
        assertEquals("signed", assistantBlocks[0].getValue("signature").jsonPrimitive.content)
        assertEquals("https://example.com", assistantBlocks[1].getValue("input").jsonObject
            .getValue("url").jsonPrimitive.content)
        assertEquals("tool-1", resultBlocks.single().getValue("tool_use_id").jsonPrimitive.content)
        assertEquals("final", (events.last() as EngineEvent.Done).content)
    }

    @Test
    fun anthropicMergesParallelToolResultsIntoOneUserTurn() = runTest {
        server.enqueue(sse("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
        gateway.stream(request(ProviderProtocol.ANTHROPIC_MESSAGES).copy(messages = listOf(
            CanonicalMessage("user", "question"),
            CanonicalMessage("assistant", toolCalls = listOf(
                CanonicalToolCall("tool-1", "first", "{}"),
                CanonicalToolCall("tool-2", "second", "{}"),
            )),
            CanonicalMessage("tool", "one", toolCallId = "tool-1"),
            CanonicalMessage("tool", "two", toolCallId = "tool-2"),
        ))).toList()

        val messages = requestBody().getValue("messages").jsonArray.map { it.jsonObject }
        assertEquals(listOf("user", "assistant", "user"), messages.map(::role))
        val results = messages.last().getValue("content").jsonArray.map { it.jsonObject }
        assertEquals(listOf("tool-1", "tool-2"), results.map {
            it.getValue("tool_use_id").jsonPrimitive.content
        })
    }

    @Test
    fun adaptersKeepToolHistoryValidBeforeANewUserTurn() = runTest {
        ProviderProtocol.entries.forEach { protocol ->
            server.enqueue(sse(when (protocol) {
                ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> "data: [DONE]\n\n"
                ProviderProtocol.OPENAI_RESPONSES ->
                    "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n"
                ProviderProtocol.ANTHROPIC_MESSAGES ->
                    "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
            }))
            gateway.stream(request(protocol).copy(messages = listOf(
                CanonicalMessage("user", "lookup"),
                CanonicalMessage("assistant", toolCalls = listOf(
                    CanonicalToolCall("call-1", "read_url", "{\"url\":\"https://example.com\"}"),
                )),
                CanonicalMessage("tool", "page contents", toolCallId = "call-1"),
                CanonicalMessage("assistant", "weather result"),
                CanonicalMessage("user", "thanks"),
            ))).toList()

            val body = requestBody()
            when (protocol) {
                ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> {
                    val messages = body.getValue("messages").jsonArray.map { it.jsonObject }
                    assertEquals(listOf("system", "user", "assistant", "tool", "assistant", "user"), messages.map(::role))
                    assertFalse("content" in messages[2])
                    assertEquals("weather result", messages[4].getValue("content").jsonPrimitive.content)
                    assertEquals("thanks", messages[5].getValue("content").jsonPrimitive.content)
                }
                ProviderProtocol.OPENAI_RESPONSES -> {
                    val input = body.getValue("input").jsonArray.map { it.jsonObject }
                    assertEquals("user", role(input[0]))
                    assertEquals("function_call", type(input[1]))
                    assertEquals("function_call_output", type(input[2]))
                    assertEquals("assistant", role(input[3]))
                    assertEquals("weather result", input[3].getValue("content").jsonPrimitive.content)
                    assertEquals("user", role(input[4]))
                    assertEquals("thanks", input[4].getValue("content").jsonPrimitive.content)
                }
                ProviderProtocol.ANTHROPIC_MESSAGES -> {
                    val messages = body.getValue("messages").jsonArray.map { it.jsonObject }
                    assertEquals(listOf("user", "assistant", "user", "assistant", "user"), messages.map(::role))
                    assertEquals("tool_use", type(messages[1].getValue("content").jsonArray.single().jsonObject))
                    assertEquals("tool_result", type(messages[2].getValue("content").jsonArray.single().jsonObject))
                    assertEquals("weather result", messages[3].getValue("content").jsonArray.single()
                        .jsonObject.getValue("text").jsonPrimitive.content)
                }
            }
        }
    }

    @Test
    fun engineRetriesOnceWithoutReasoningOnValidationError() = runTest {
        server.enqueue(MockResponse().setResponseCode(422).setBody("{\"error\":{\"message\":\"reasoning unsupported\"}}"))
        server.enqueue(sse("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
        val engine = DirectChatEngine(gateway, NoTools)

        val events = engine.run(request(ProviderProtocol.OPENAI_RESPONSES), false, false, 0).toList()
        val firstBody = server.takeRequest().body.readUtf8()
        val secondBody = server.takeRequest().body.readUtf8()

        assertTrue(firstBody.contains("\"reasoning\""))
        assertFalse(secondBody.contains("\"reasoning\""))
        assertEquals("ok", (events.last() as EngineEvent.Done).content)
    }

    @Test
    fun engineReturnsToolResultsAndRetriesOnlyBeforeProviderContent() = runTest {
        server.enqueue(MockResponse().setResponseCode(503).setBody("unavailable"))
        server.enqueue(sse(
            "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-tool\",\"function\":{\"name\":\"read_url\",\"arguments\":\"{\\\"url\\\":\\\"https://example.com\\\"}\"}}]}}]}\n\n" +
                "data: [DONE]\n\n",
        ))
        server.enqueue(sse("data: {\"choices\":[{\"delta\":{\"content\":\"final\"}}]}\n\ndata: [DONE]\n\n"))
        val engine = DirectChatEngine(gateway, FakeTools)

        val events = engine.run(request(ProviderProtocol.OPENAI_CHAT_COMPLETIONS), false, true, 7).toList()
        val retried = server.takeRequest()
        val toolRequest = server.takeRequest()
        val finalRequest = server.takeRequest()

        assertEquals("/v1/chat/completions", retried.path)
        assertEquals(retried.getHeader("X-Client-Request-Id"), toolRequest.getHeader("X-Client-Request-Id"))
        assertEquals(toolRequest.getHeader("X-Client-Request-Id"), finalRequest.getHeader("X-Client-Request-Id"))
        assertTrue(toolRequest.body.readUtf8().contains("read_url"))
        val finalBody = finalRequest.body.readUtf8()
        assertTrue(finalBody.contains("tool_call_id"))
        assertTrue(finalBody.contains("page contents"))
        val messages = JSON.parseToJsonElement(finalBody).jsonObject.getValue("messages").jsonArray
            .map { it.jsonObject }
        val toolCallingAssistant = messages.first { "tool_calls" in it }
        assertFalse("content" in toolCallingAssistant)
        assertEquals("final", (events.last() as EngineEvent.Done).content)
        assertTrue(events.any { it is EngineEvent.Process && it.event.type == "tool_completed" })
    }

    private fun request(protocol: ProviderProtocol) = ModelCallRequest(
        model = ModelProfile(providerId = "provider", remoteId = "model-a", displayName = "Model A"),
        provider = provider(protocol),
        apiKey = "secret",
        systemPrompt = "system",
        thinkingEffort = "medium",
        messages = listOf(CanonicalMessage("user", "hello")),
        tools = emptyList(),
        requestId = "request-1",
    )

    private fun provider(protocol: ProviderProtocol) = ProviderConfig(
        id = "provider",
        name = "Test",
        baseUrl = server.url("/v1").toString().trimEnd('/'),
        protocol = protocol,
        apiKeyConfigured = true,
    )

    private fun sse(body: String) = MockResponse()
        .setHeader("Content-Type", "text/event-stream")
        .setBody(body)

    private fun json(body: String) = MockResponse()
        .setHeader("Content-Type", "application/json")
        .setBody(body)

    private fun requestBody(): JsonObject = JSON.parseToJsonElement(server.takeRequest().body.readUtf8()).jsonObject

    private fun role(value: JsonObject): String = value.getValue("role").jsonPrimitive.content

    private fun type(value: JsonObject): String = value.getValue("type").jsonPrimitive.content

    private object NoTools : ToolRunner {
        override fun definitions(enableSearch: Boolean, enableRead: Boolean) = emptyList<ToolDefinition>()
        override suspend fun execute(call: CanonicalToolCall, enableSearch: Boolean, enableRead: Boolean) =
            ToolExecutionResult("{}", true)
    }

    private object FakeTools : ToolRunner {
        override fun definitions(enableSearch: Boolean, enableRead: Boolean) = if (enableRead) listOf(
            ToolDefinition("read_url", "Read URL", JsonObject(emptyMap())),
        ) else emptyList()

        override suspend fun execute(call: CanonicalToolCall, enableSearch: Boolean, enableRead: Boolean) =
            ToolExecutionResult("page contents", true)
    }

    companion object {
        private val JSON = Json { ignoreUnknownKeys = true }
    }
}
