package io.nullwake.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import io.nullwake.app.ui.components.DeviceSwitcherTopBar
import io.nullwake.app.ui.i18n.LocalAppStrings
import io.nullwake.app.ui.i18n.resolveAppStrings
import io.nullwake.app.ui.screens.DashboardScreen
import io.nullwake.app.ui.screens.SettingsScreen
import io.nullwake.app.ui.theme.NullWakeTheme
import io.nullwake.app.ui.theme.ThemeMode
import java.io.File

class MainActivity : ComponentActivity() {
    private val viewModel: NullWakeViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        val crashFile = File(filesDir, "last_crash.txt")
        val prevHandler = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, throwable ->
            runCatching {
                crashFile.writeText("Thread ${thread.name}: ${throwable.stackTraceToString()}")
            }
            prevHandler?.uncaughtException(thread, throwable)
        }

        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            val uiState by viewModel.uiState.collectAsState()
            val appStrings = remember(uiState.language) { resolveAppStrings(uiState.language) }
            val isDark = when (uiState.themeMode) {
                ThemeMode.SYSTEM -> isSystemInDarkTheme()
                ThemeMode.LIGHT -> false
                ThemeMode.DARK -> true
            }

            val view = LocalView.current
            if (!view.isInEditMode) {
                SideEffect {
                    val insetsController = WindowCompat.getInsetsController(window, view)
                    insetsController.isAppearanceLightStatusBars = !isDark
                    insetsController.isAppearanceLightNavigationBars = !isDark
                }
            }

            CompositionLocalProvider(LocalAppStrings provides appStrings) {
                NullWakeTheme(
                    themeMode = uiState.themeMode,
                    amoledBlack = uiState.amoledBlack,
                    dynamicColor = uiState.dynamicColor,
                ) {
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
                        containerColor = MaterialTheme.colorScheme.background,
                        topBar = {
                            DeviceSwitcherTopBar(
                                nodes = uiState.nodes,
                                selectedNode = uiState.selectedNode,
                                customNames = uiState.customNames,
                                isSettingsOpen = showSettings,
                                onSelectNode = { node -> viewModel.selectNode(node) },
                                onRenameNode = { id, newName -> viewModel.setCustomNodeName(id, newName) },
                                onRefreshNodes = { viewModel.refreshNodes() },
                                onToggleSettings = { showSettings = !showSettings },
                            )
                        },
                    ) { innerPadding ->
                        AnimatedContent(
                            targetState = showSettings,
                            transitionSpec = {
                                if (targetState) {
                                    (fadeIn(tween(240)) + slideInHorizontally(tween(260)) { it / 12 }) togetherWith
                                        (fadeOut(tween(180)) + slideOutHorizontally(tween(220)) { -it / 16 })
                                } else {
                                    (fadeIn(tween(240)) + slideInHorizontally(tween(260)) { -it / 12 }) togetherWith
                                        (fadeOut(tween(180)) + slideOutHorizontally(tween(220)) { it / 16 })
                                }
                            },
                            label = "screen_transition",
                        ) { settingsVisible ->
                            if (settingsVisible) {
                                SettingsScreen(
                                    uiState = uiState,
                                    onSelectThemeMode = { viewModel.setThemeMode(it) },
                                    onToggleAmoled = { viewModel.setAmoledBlack(it) },
                                    onToggleDynamicColor = { viewModel.setDynamicColor(it) },
                                    onSelectLanguage = { viewModel.setAppLanguage(it) },
                                    onRenameNode = { id, newName -> viewModel.setCustomNodeName(id, newName) },
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
    }
}
