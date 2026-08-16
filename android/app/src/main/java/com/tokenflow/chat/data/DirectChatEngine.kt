package com.tokenflow.chat.data

import java.io.IOException
import java.util.UUID
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.flow

class DirectChatEngine(
    private val gateway: ModelGateway,
    private val tools: ToolRunner,
) {
    fun run(
        initial: ModelCallRequest,
        enableSearch: Boolean,
        enableRead: Boolean,
        maxToolCalls: Int,
        urlReaderBackend: UrlReaderBackend = UrlReaderBackend.BUILT_IN,
    ): Flow<EngineEvent> = run(
        initial,
        ToolOptions(enableSearch, enableRead, urlReaderBackend = urlReaderBackend),
        maxToolCalls,
    )

    fun run(
        initial: ModelCallRequest,
        options: ToolOptions,
        maxToolCalls: Int,
    ): Flow<EngineEvent> = flow {
        val transcript = initial.messages.toMutableList()
        val output = StringBuilder()
        val processEvents = mutableListOf<ProcessEvent>()
        var totalUsage = Usage()
        var usedToolCalls = 0
        var thinkingEffort = initial.thinkingEffort
        var round = 0

        while (true) {
            round += 1
            val definitions = if (usedToolCalls < maxToolCalls.coerceIn(0, 20)) {
                tools.definitions(options)
            } else {
                emptyList()
            }
            val responseText = StringBuilder()
            val calls = linkedMapOf<Int, MutableToolCall>()
            val replayItems = linkedMapOf<Int, ProtocolReplayItem>()
            var receivedProviderContent = false
            var transportAttempt = 0
            var thinkingFallbackUsed = false

            while (true) {
                val callRequest = initial.copy(
                    thinkingEffort = thinkingEffort,
                    messages = transcript.toList(),
                    tools = definitions,
                )
                try {
                    gateway.stream(callRequest).collect { event ->
                        when (event) {
                            is ModelStreamEvent.TextDelta -> if (event.content.isNotEmpty()) {
                                receivedProviderContent = true
                                responseText.append(event.content)
                                output.append(event.content)
                                emit(EngineEvent.Delta(event.content))
                            }
                            is ModelStreamEvent.ThinkingDelta -> if (event.content.isNotEmpty()) {
                                receivedProviderContent = true
                                val process = ProcessEvent(
                                    type = "thinking",
                                    id = "thinking-$round",
                                    content = event.content,
                                )
                                mergeProcess(processEvents, process)
                                emit(EngineEvent.Process(process))
                            }
                            is ModelStreamEvent.ToolCallDelta -> {
                                if (event.id.isNotEmpty() || event.name.isNotEmpty() || event.arguments.isNotEmpty()) {
                                    receivedProviderContent = true
                                }
                                val call = calls.getOrPut(event.index) { MutableToolCall() }
                                if (event.id.isNotEmpty()) call.id = event.id
                                if (event.name.isNotEmpty()) call.name = event.name
                                if (event.replaceArguments) call.arguments.clear()
                                call.arguments.append(event.arguments)
                            }
                            is ModelStreamEvent.ReplayItem -> {
                                receivedProviderContent = true
                                replayItems[event.item.index] = event.item
                            }
                            is ModelStreamEvent.TokenUsage -> totalUsage += event.usage
                            ModelStreamEvent.Completed -> Unit
                        }
                    }
                    break
                } catch (cancelled: CancellationException) {
                    throw cancelled
                } catch (error: ApiException) {
                    if (!receivedProviderContent && !thinkingFallbackUsed && thinkingEffort != "off" &&
                        error.status in setOf(400, 422)
                    ) {
                        thinkingFallbackUsed = true
                        thinkingEffort = "off"
                        val event = ProcessEvent(type = "status", message = "Retrying without reasoning parameters")
                        processEvents += event
                        emit(EngineEvent.Process(event))
                        continue
                    }
                    if (!receivedProviderContent && error.retryable && transportAttempt < MAX_TRANSPORT_RETRIES) {
                        transportAttempt += 1
                        val event = retryEvent(transportAttempt)
                        processEvents += event
                        emit(EngineEvent.Process(event))
                        delay(retryDelayMillis(transportAttempt))
                        continue
                    }
                    throw error
                } catch (error: IOException) {
                    if (!receivedProviderContent && transportAttempt < MAX_TRANSPORT_RETRIES) {
                        transportAttempt += 1
                        val event = retryEvent(transportAttempt)
                        processEvents += event
                        emit(EngineEvent.Process(event))
                        delay(retryDelayMillis(transportAttempt))
                        continue
                    }
                    throw error
                }
            }

            val completedCalls = calls.toSortedMap().values.mapNotNull { it.complete() }
            if (responseText.isNotEmpty() || completedCalls.isNotEmpty() || replayItems.isNotEmpty()) {
                transcript += CanonicalMessage(
                    role = "assistant",
                    content = responseText.toString(),
                    toolCalls = completedCalls,
                    replayItems = replayItems.toSortedMap().values.toList(),
                )
            }
            if (completedCalls.isEmpty()) break

            completedCalls.forEach { call ->
                val canExecute = usedToolCalls < maxToolCalls.coerceIn(0, 20)
                val started = ProcessEvent(
                    type = "tool_started",
                    id = call.id,
                    name = call.name,
                    arguments = call.arguments,
                    ok = canExecute,
                )
                processEvents += started
                emit(EngineEvent.Process(started))

                val result = if (canExecute) {
                    usedToolCalls += 1
                    tools.execute(call, options)
                } else {
                    ToolExecutionResult("{\"error\":\"Maximum tool call limit reached\"}", false)
                }
                result.processEvent?.let { diagnostic ->
                    processEvents += diagnostic
                    emit(EngineEvent.Process(diagnostic))
                }
                val completed = ProcessEvent(
                    type = if (result.ok) "tool_completed" else "tool_failed",
                    id = call.id,
                    name = call.name,
                    arguments = call.arguments,
                    result = result.content,
                    ok = result.ok,
                )
                processEvents += completed
                emit(EngineEvent.Process(completed))
                transcript += CanonicalMessage(
                    role = "tool",
                    content = result.content,
                    toolCallId = call.id,
                    toolName = call.name,
                )
            }
        }

        emit(EngineEvent.Done(output.toString(), totalUsage, processEvents.toList()))
    }

    suspend fun generateTitle(request: ModelCallRequest): String {
        val output = StringBuilder()
        gateway.stream(
            request.copy(
                thinkingEffort = "off",
                tools = emptyList(),
                maxOutputTokens = 80,
                requestId = UUID.randomUUID().toString(),
            ),
        ).collect { event ->
            if (event is ModelStreamEvent.TextDelta && output.length < 160) output.append(event.content)
        }
        return output.toString().trim().trim('"', '\'', '`').lineSequence().firstOrNull().orEmpty().take(120)
    }

    private fun retryEvent(attempt: Int) = ProcessEvent(
        type = "status",
        messageKey = "retrying",
        attempt = attempt,
        maxAttempts = MAX_TRANSPORT_RETRIES,
    )

    private fun retryDelayMillis(attempt: Int): Long = 600L * attempt.coerceIn(1, MAX_TRANSPORT_RETRIES)

    private fun mergeProcess(events: MutableList<ProcessEvent>, incoming: ProcessEvent) {
        val index = events.indexOfLast { it.type == "thinking" && it.id == incoming.id }
        if (index < 0) events += incoming
        else events[index] = events[index].copy(content = events[index].content + incoming.content)
    }

    private data class MutableToolCall(
        var id: String = "",
        var name: String = "",
        val arguments: StringBuilder = StringBuilder(),
    ) {
        fun complete(): CanonicalToolCall? {
            if (name.isBlank()) return null
            return CanonicalToolCall(id.ifBlank { UUID.randomUUID().toString() }, name, arguments.toString().ifBlank { "{}" })
        }
    }

    companion object {
        const val MAX_TRANSPORT_RETRIES = 2
    }
}
