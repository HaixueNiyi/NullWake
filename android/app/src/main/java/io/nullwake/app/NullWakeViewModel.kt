package io.nullwake.app

import android.app.Application
import android.content.Context
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import io.nullwake.app.bridge.NullWakeBridge
import io.nullwake.app.model.PeerNode
import io.nullwake.app.model.TelemetryMessage
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File

data class NullWakeUiState(
    val tailscaleState: String = "Stopped",
    val authUrl: String? = null,
    val nodes: List<PeerNode> = emptyList(),
    val selectedNode: PeerNode? = null,
    val isSubscribed: Boolean = false,
    val telemetry: TelemetryMessage? = null,
    val cpuHistory: List<Float> = emptyList(),
    val ramHistory: List<Float> = emptyList(),
    val netRxHistory: List<Float> = emptyList(),
    val netTxHistory: List<Float> = emptyList(),
    val gpuHistory: List<Float> = emptyList(),
    val intervalMs: Long = 1000L,
    val proxyUrl: String = "",
    val authKey: String = "",
    val errorMessage: String? = null,
)

class NullWakeViewModel(application: Application) : AndroidViewModel(application) {

    private val prefs = application.getSharedPreferences("nullwake_prefs", Context.MODE_PRIVATE)
    private val stateDir = File(application.filesDir, "tsnet-state").apply { mkdirs() }
    private val bridge = NullWakeBridge()

    private val _uiState = MutableStateFlow(
        NullWakeUiState(
            intervalMs = prefs.getLong(KEY_INTERVAL_MS, 1000L),
            proxyUrl = prefs.getString(KEY_PROXY_URL, "") ?: "",
            authKey = prefs.getString(KEY_AUTH_KEY, "") ?: "",
        ),
    )
    val uiState: StateFlow<NullWakeUiState> = _uiState.asStateFlow()

    @Volatile
    private var isForeground = false
    private var nodePollJob: Job? = null

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
                        _uiState.update { it.copy(errorMessage = error, isSubscribed = false) }
                    }
                },
            )

            result.onFailure { err ->
                _uiState.update {
                    it.copy(
                        tailscaleState = "Stopped",
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
        viewModelScope.launch(Dispatchers.IO) {
            bridge.unsubscribe()
            _uiState.update { it.copy(isSubscribed = false) }
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
        viewModelScope.launch(Dispatchers.IO) {
            bridge.unsubscribe()
            val addr = "${node.ip}:9100"
            bridge.subscribeNode(addr, intervalMs)
                .onSuccess {
                    _uiState.update { it.copy(isSubscribed = true, errorMessage = null) }
                }
                .onFailure { err ->
                    _uiState.update {
                        it.copy(
                            isSubscribed = false,
                            errorMessage = err.message ?: "Failed to connect to $addr",
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
                delay(5000L)
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

    override fun onCleared() {
        super.onCleared()
        bridge.stop()
    }

    private companion object {
        const val MAX_HISTORY_POINTS = 60
        const val KEY_INTERVAL_MS = "interval_ms"
        const val KEY_PROXY_URL = "proxy_url"
        const val KEY_AUTH_KEY = "auth_key"
        const val KEY_LAST_NODE_ID = "last_node_id"

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
