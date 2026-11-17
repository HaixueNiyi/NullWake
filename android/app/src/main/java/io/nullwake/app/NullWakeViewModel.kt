package io.nullwake.app

import android.app.Application
import android.content.Context
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import io.nullwake.app.bridge.NullWakeBridge
import io.nullwake.app.model.PeerNode
import io.nullwake.app.model.TelemetryMessage
import io.nullwake.app.ui.i18n.AppLanguage
import io.nullwake.app.ui.theme.ThemeMode
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.io.File

data class NullWakeUiState(
    val tailscaleState: String = "Stopped",
    val authUrl: String? = null,
    val nodes: List<PeerNode> = emptyList(),
    val selectedNode: PeerNode? = null,
    val customNames: Map<String, String> = emptyMap(),
    val isSubscribed: Boolean = false,
    val isConnecting: Boolean = false,
    val telemetry: TelemetryMessage? = null,
    val cpuHistory: List<Float> = emptyList(),
    val ramHistory: List<Float> = emptyList(),
    val netRxHistory: List<Float> = emptyList(),
    val netTxHistory: List<Float> = emptyList(),
    val gpuHistory: List<Float> = emptyList(),
    val intervalMs: Long = 1000L,
    val proxyUrl: String = "",
    val authKey: String = "",
    val themeMode: ThemeMode = ThemeMode.SYSTEM,
    val amoledBlack: Boolean = false,
    val dynamicColor: Boolean = true,
    val language: AppLanguage = AppLanguage.SYSTEM,
    val errorMessage: String? = null,
) {
    fun displayNameFor(node: PeerNode): String {
        val custom = customNames[node.id]?.trim()
        return if (!custom.isNullOrEmpty()) custom else node.name
    }
}

class NullWakeViewModel(application: Application) : AndroidViewModel(application) {

    private val prefs = application.getSharedPreferences("nullwake_prefs", Context.MODE_PRIVATE)
    private val stateDir = File(application.filesDir, "tsnet-state").apply { mkdirs() }
    private val bridge = NullWakeBridge()

    private val _uiState = MutableStateFlow(
        NullWakeUiState(
            customNames = loadCustomNames(),
            intervalMs = prefs.getLong(KEY_INTERVAL_MS, 1000L),
            proxyUrl = prefs.getString(KEY_PROXY_URL, "") ?: "",
            authKey = prefs.getString(KEY_AUTH_KEY, "") ?: "",
            themeMode = ThemeMode.fromKey(prefs.getString(KEY_THEME_MODE, null)),
            amoledBlack = prefs.getBoolean(KEY_AMOLED_BLACK, false),
            dynamicColor = prefs.getBoolean(KEY_DYNAMIC_COLOR, true),
            language = AppLanguage.fromKey(prefs.getString(KEY_LANGUAGE, null)),
        ),
    )
    val uiState: StateFlow<NullWakeUiState> = _uiState.asStateFlow()

    @Volatile
    private var isForeground = false
    private var nodePollJob: Job? = null
    private var subscribeJob: Job? = null

    init {
        val crashFile = File(application.filesDir, "last_crash.txt")
        if (crashFile.exists()) {
            val trace = runCatching { crashFile.readText() }.getOrNull()
            crashFile.delete()
            if (!trace.isNullOrBlank()) {
                _uiState.update { it.copy(errorMessage = "Previous crash:\n${trace.take(1200)}") }
            }
        }
        startBridge()
    }

    fun startBridge(clearState: Boolean = false) {
        subscribeJob?.cancel()
        viewModelScope.launch(Dispatchers.IO) {
            bridge.stop()
            if (clearState) {
                stateDir.deleteRecursively()
                stateDir.mkdirs()
            }

            val snapshot = _uiState.value
            _uiState.update {
                it.copy(
                    tailscaleState = "Starting",
                    authUrl = null,
                    errorMessage = null,
                    isSubscribed = false,
                    isConnecting = false,
                )
            }

            val result = bridge.start(
                stateDir = stateDir.absolutePath,
                authKey = snapshot.authKey.trim(),
                proxyUrl = snapshot.proxyUrl.trim(),
                callback = object : NullWakeBridge.Callback {
                    override fun onAuthURL(url: String) {
                        _uiState.update { it.copy(authUrl = url, errorMessage = null) }
                    }

                    override fun onStateChange(state: String) {
                        if (state == "PeersUpdated") {
                            refreshNodesInternal()
                            return
                        }
                        _uiState.update {
                            it.copy(
                                tailscaleState = state,
                                authUrl = if (state == "Running") null else it.authUrl,
                            )
                        }
                        if (state == "Running") {
                            refreshNodesInternal()
                        }
                    }

                    override fun onTelemetry(message: TelemetryMessage) {
                        val data = message.data
                        _uiState.update { cur ->
                            cur.copy(
                                telemetry = message,
                                isSubscribed = true,
                                isConnecting = false,
                                errorMessage = null,
                                cpuHistory = appendRolling(cur.cpuHistory, data.cpu.usagePercent.toFloat()),
                                ramHistory = appendRolling(cur.ramHistory, data.ram.usagePercent),
                                netRxHistory = appendRolling(cur.netRxHistory, data.net.rxBytesSec / 1024f),
                                netTxHistory = appendRolling(cur.netTxHistory, data.net.txBytesSec / 1024f),
                                gpuHistory = if (data.gpu != null) {
                                    appendRolling(cur.gpuHistory, data.gpu.usagePercent.toFloat())
                                } else {
                                    emptyList()
                                },
                            )
                        }
                    }

                    override fun onError(error: String) {
                        _uiState.update {
                            if (it.isConnecting) it
                            else it.copy(errorMessage = error, isSubscribed = false)
                        }
                    }
                },
            )

            result.onFailure { err ->
                _uiState.update {
                    it.copy(
                        tailscaleState = "Stopped",
                        isConnecting = false,
                        errorMessage = err.message ?: "Failed to start tsnet bridge",
                    )
                }
            }
        }
    }

    fun onAppForeground() {
        isForeground = true
        startNodePolling()
        val node = _uiState.value.selectedNode
        if (node != null && _uiState.value.tailscaleState == "Running") {
            subscribeToNode(node)
        }
    }

    fun onAppBackground() {
        isForeground = false
        nodePollJob?.cancel()
        nodePollJob = null
        subscribeJob?.cancel()
        subscribeJob = null
        viewModelScope.launch(Dispatchers.IO) {
            bridge.unsubscribe()
            _uiState.update { it.copy(isSubscribed = false, isConnecting = false) }
        }
    }

    fun refreshNodes() {
        viewModelScope.launch(Dispatchers.IO) {
            refreshNodesInternal()
        }
    }

    fun selectNode(node: PeerNode) {
        prefs.edit().putString(KEY_LAST_NODE_ID, node.id).apply()
        _uiState.update {
            it.copy(
                selectedNode = node,
                telemetry = null,
                cpuHistory = emptyList(),
                ramHistory = emptyList(),
                netRxHistory = emptyList(),
                netTxHistory = emptyList(),
                gpuHistory = emptyList(),
                errorMessage = null,
            )
        }
        if (isForeground) {
            subscribeToNode(node)
        }
    }

    fun setCustomNodeName(nodeId: String, customName: String) {
        val trimmed = customName.trim()
        val updated = _uiState.value.customNames.toMutableMap()
        if (trimmed.isEmpty()) {
            updated.remove(nodeId)
        } else {
            updated[nodeId] = trimmed
        }
        saveCustomNames(updated)
        _uiState.update { it.copy(customNames = updated) }
    }

    fun setThemeMode(mode: ThemeMode) {
        prefs.edit().putString(KEY_THEME_MODE, mode.key).apply()
        _uiState.update { it.copy(themeMode = mode) }
    }

    fun setAmoledBlack(enabled: Boolean) {
        prefs.edit().putBoolean(KEY_AMOLED_BLACK, enabled).apply()
        _uiState.update { it.copy(amoledBlack = enabled) }
    }

    fun setDynamicColor(enabled: Boolean) {
        prefs.edit().putBoolean(KEY_DYNAMIC_COLOR, enabled).apply()
        _uiState.update { it.copy(dynamicColor = enabled) }
    }

    fun setAppLanguage(language: AppLanguage) {
        prefs.edit().putString(KEY_LANGUAGE, language.key).apply()
        _uiState.update { it.copy(language = language) }
    }

    fun setPollingInterval(intervalMs: Long) {
        prefs.edit().putLong(KEY_INTERVAL_MS, intervalMs).apply()
        _uiState.update { it.copy(intervalMs = intervalMs) }
        val node = _uiState.value.selectedNode
        if (isForeground && node != null && _uiState.value.isSubscribed) {
            subscribeToNode(node)
        }
    }

    fun applyConnectionSettings(proxyUrl: String, authKey: String) {
        prefs.edit()
            .putString(KEY_PROXY_URL, proxyUrl.trim())
            .putString(KEY_AUTH_KEY, authKey.trim())
            .apply()
        _uiState.update {
            it.copy(
                proxyUrl = proxyUrl.trim(),
                authKey = authKey.trim(),
            )
        }
        startBridge(clearState = false)
    }

    fun resetTailscaleLogin() {
        prefs.edit().remove(KEY_LAST_NODE_ID).apply()
        _uiState.update {
            it.copy(
                nodes = emptyList(),
                selectedNode = null,
                telemetry = null,
            )
        }
        startBridge(clearState = true)
    }

    private fun subscribeToNode(node: PeerNode) {
        val intervalMs = _uiState.value.intervalMs
        subscribeJob?.cancel()
        subscribeJob = viewModelScope.launch(Dispatchers.IO) {
            _uiState.update { it.copy(isConnecting = true, errorMessage = null) }
            bridge.unsubscribe()
            val addr = "${node.ip}:9100"

            var lastError: Throwable? = null
            for (attempt in 0..1) {
                if (!isActive) return@launch
                val res = bridge.subscribeNode(addr, intervalMs)
                if (res.isSuccess) {
                    _uiState.update {
                        it.copy(
                            isSubscribed = true,
                            isConnecting = false,
                            errorMessage = null,
                        )
                    }
                    return@launch
                }
                lastError = res.exceptionOrNull()
                if (attempt == 0 && isActive) {
                    delay(1000L)
                }
            }

            if (isActive) {
                _uiState.update {
                    it.copy(
                        isSubscribed = false,
                        isConnecting = false,
                        errorMessage = lastError?.message ?: "Failed to connect to $addr",
                    )
                }
            }
        }
    }

    private fun startNodePolling() {
        nodePollJob?.cancel()
        nodePollJob = viewModelScope.launch(Dispatchers.IO) {
            while (isActive) {
                if (_uiState.value.tailscaleState == "Running") {
                    refreshNodesInternal()
                }
                val interval = if (_uiState.value.nodes.isEmpty()) 500L else 5000L
                delay(interval)
            }
        }
    }

    private fun refreshNodesInternal() {
        val discovered = bridge.getOnlineNodes()
        val lastNodeId = prefs.getString(KEY_LAST_NODE_ID, null)
        var newlySelected: PeerNode? = null

        _uiState.update { cur ->
            val currentSelected = cur.selectedNode?.let { sel ->
                discovered.firstOrNull { it.id == sel.id } ?: sel
            } ?: discovered.firstOrNull { it.id == lastNodeId }
                ?: discovered.firstOrNull { it.online }

            if (cur.selectedNode == null && currentSelected != null) {
                newlySelected = currentSelected
            }

            cur.copy(
                nodes = discovered,
                selectedNode = currentSelected,
            )
        }

        if (newlySelected != null && isForeground) {
            subscribeToNode(newlySelected!!)
        }
    }

    private fun loadCustomNames(): Map<String, String> {
        val raw = prefs.getString(KEY_CUSTOM_NAMES_JSON, null) ?: return emptyMap()
        return runCatching {
            val obj = JSONObject(raw)
            buildMap {
                val keys = obj.keys()
                while (keys.hasNext()) {
                    val k = keys.next()
                    val v = obj.optString(k, "").trim()
                    if (v.isNotEmpty()) put(k, v)
                }
            }
        }.getOrDefault(emptyMap())
    }

    private fun saveCustomNames(names: Map<String, String>) {
        val obj = JSONObject()
        names.forEach { (k, v) -> obj.put(k, v) }
        prefs.edit().putString(KEY_CUSTOM_NAMES_JSON, obj.toString()).apply()
    }

    override fun onCleared() {
        super.onCleared()
        subscribeJob?.cancel()
        bridge.stop()
    }

    private companion object {
        const val MAX_HISTORY_POINTS = 60
        const val KEY_INTERVAL_MS = "interval_ms"
        const val KEY_PROXY_URL = "proxy_url"
        const val KEY_AUTH_KEY = "auth_key"
        const val KEY_LAST_NODE_ID = "last_node_id"
        const val KEY_CUSTOM_NAMES_JSON = "custom_names_json"
        const val KEY_THEME_MODE = "theme_mode"
        const val KEY_AMOLED_BLACK = "amoled_black"
        const val KEY_DYNAMIC_COLOR = "dynamic_color"
        const val KEY_LANGUAGE = "app_language"

        fun appendRolling(existing: List<Float>, next: Float): List<Float> {
            val result = ArrayList<Float>(minOf(existing.size + 1, MAX_HISTORY_POINTS))
            val start = if (existing.size >= MAX_HISTORY_POINTS) existing.size - (MAX_HISTORY_POINTS - 1) else 0
            for (i in start until existing.size) {
                result.add(existing[i])
            }
            result.add(next)
            return result
        }
    }
}
