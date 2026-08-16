package com.tokenflow.chat

import android.app.Application
import com.tokenflow.chat.data.AppContainer

class TokenFlowApplication : Application() {
    val container: AppContainer by lazy { AppContainer(this) }
}
