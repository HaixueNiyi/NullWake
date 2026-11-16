package io.nullwake.app.bridge

import io.nullwake.app.model.PeerNode
import io.nullwake.app.model.TelemetryMessage
import io.nullwake.app.model.TelemetryParser
import tsnetbridge.BridgeListener
import tsnetbridge.Tsnetbridge

/**
 * Kotlin wrapper around the gomobile-compiled tsnet-bridge.aar.
 */
class NullWakeBridge {
    interface Callback {
        fun onAuthURL(url: String)
        fun onStateChange(state: String)
        fun onTelemetry(message: TelemetryMessage)
        fun onError(error: String)
    }

    fun start(
        stateDir: String,
        authKey: String,
        proxyUrl: String,
        callback: Callback,
    ): Result<Unit> = runCatching {
        val listener = object : BridgeListener {
            override fun onAuthURL(url: String?) {
                if (!url.isNullOrBlank()) {
                    callback.onAuthURL(url)
                }
            }

            override fun onStateChange(state: String?) {
                if (!state.isNullOrBlank()) {
                    callback.onStateChange(state)
                }
            }

            override fun onTelemetry(jsonPayload: String?) {
                if (jsonPayload.isNullOrBlank()) return
                TelemetryParser.parseTelemetry(jsonPayload)?.let(callback::onTelemetry)
            }

            override fun onError(errMsg: String?) {
                if (!errMsg.isNullOrBlank()) {
                    callback.onError(errMsg)
                }
            }
        }
        Tsnetbridge.start(stateDir, authKey, proxyUrl, listener)
    }

    fun getOnlineNodes(): List<PeerNode> {
        return runCatching {
            TelemetryParser.parsePeerNodes(Tsnetbridge.getOnlineNodes())
        }.getOrDefault(emptyList())
    }

    fun subscribeNode(targetAddr: String, intervalMs: Long): Result<Unit> = runCatching {
        Tsnetbridge.subscribeNode(targetAddr, intervalMs)
    }

    fun unsubscribe() {
        runCatching { Tsnetbridge.unsubscribe() }
    }

    fun stop() {
        runCatching { Tsnetbridge.stop() }
    }
}
