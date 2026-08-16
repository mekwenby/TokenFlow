package com.tokenflow.chat.data

import android.content.Context

class AppContainer(context: Context) {
    @Suppress("unused")
    private val cameraCaptureStore = CameraCaptureStore(context)
    val json = DirectApiTransport.defaultJson
    private val database = TokenFlowDatabase.open(context)
    private val secrets = SecretStore(context)
    private val gateway = ModelGateway(DirectApiTransport(json), json)
    private val knowledgeStore = KnowledgeStore(context, database.localDao())
    private val exaClient = ExaClient(json)
    private val builtInUrlReader = UrlReader(context, json)
    private val webTools = WebToolExecutor(
        secretStore = secrets,
        exaClient = exaClient,
        urlReader = builtInUrlReader,
        json = json,
        infoFlowReader = InfoFlowUrlReader(builtIn = builtInUrlReader, json = json),
        knowledgeStore = knowledgeStore,
    )
    private val engine = DirectChatEngine(gateway, webTools)
    val repository: ChatDataSource = ChatRepository(
        dao = database.localDao(),
        secretStore = secrets,
        gateway = gateway,
        engine = engine,
        archive = ConfigArchiveCodec(json),
        json = json,
        avatarStore = LocalAvatarStore(context),
        knowledgeStore = knowledgeStore,
        exaClient = exaClient,
        attachmentStore = AttachmentStore(context, database.localDao()),
        mimoTtsClient = MimoTtsClient(context, secrets, json),
        infoFlowReader = InfoFlowUrlReader(builtIn = builtInUrlReader, json = json),
    )
}
