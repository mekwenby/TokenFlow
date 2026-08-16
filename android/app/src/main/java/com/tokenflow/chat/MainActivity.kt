package com.tokenflow.chat

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import com.tokenflow.chat.ui.AppViewModel
import com.tokenflow.chat.ui.AppViewModelFactory
import com.tokenflow.chat.ui.TokenFlowApp
import com.tokenflow.chat.ui.theme.TokenFlowTheme

class MainActivity : ComponentActivity() {
    private val viewModel: AppViewModel by viewModels {
        val container = (application as TokenFlowApplication).container
        AppViewModelFactory(container.repository)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            TokenFlowTheme {
                TokenFlowApp(viewModel)
            }
        }
    }
}
