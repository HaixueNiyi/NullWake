package io.nullwake.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import io.nullwake.app.ui.components.DeviceSwitcherTopBar
import io.nullwake.app.ui.screens.DashboardScreen
import io.nullwake.app.ui.screens.SettingsScreen
import io.nullwake.app.ui.theme.NullWakeColors
import io.nullwake.app.ui.theme.NullWakeTheme

class MainActivity : ComponentActivity() {
    private val viewModel: NullWakeViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            NullWakeTheme {
                val uiState by viewModel.uiState.collectAsState()
                var showSettings by remember { mutableStateOf(false) }

                val lifecycleOwner = LocalLifecycleOwner.current
                DisposableEffect(lifecycleOwner) {
                    val observer = LifecycleEventObserver { _, event ->
                        when (event) {
                            Lifecycle.Event.ON_START -> viewModel.onAppForeground()
                            Lifecycle.Event.ON_STOP -> viewModel.onAppBackground()
                            else -> Unit
                        }
                    }
                    lifecycleOwner.lifecycle.addObserver(observer)
                    onDispose {
                        lifecycleOwner.lifecycle.removeObserver(observer)
                    }
                }

                val openAuthBrowser: (String) -> Unit = { url ->
                    runCatching {
                        val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url))
                        startActivity(intent)
                    }
                }

                Scaffold(
                    containerColor = NullWakeColors.Background,
                    topBar = {
                        DeviceSwitcherTopBar(
                            nodes = uiState.nodes,
                            selectedNode = uiState.selectedNode,
                            isSettingsOpen = showSettings,
                            onSelectNode = { node -> viewModel.selectNode(node) },
                            onRefreshNodes = { viewModel.refreshNodes() },
                            onToggleSettings = { showSettings = !showSettings },
                        )
                    },
                ) { innerPadding ->
                    if (showSettings) {
                        SettingsScreen(
                            uiState = uiState,
                            onSelectInterval = { viewModel.setPollingInterval(it) },
                            onApplyConnectionSettings = { proxy, key ->
                                viewModel.applyConnectionSettings(proxy, key)
                            },
                            onOpenAuthUrl = openAuthBrowser,
                            onResetLogin = { viewModel.resetTailscaleLogin() },
                            modifier = Modifier.padding(innerPadding),
                        )
                    } else {
                        DashboardScreen(
                            uiState = uiState,
                            onOpenAuthUrl = openAuthBrowser,
                            onOpenSettings = { showSettings = true },
                            modifier = Modifier.padding(innerPadding),
                        )
                    }
                }
            }
        }
    }
}
