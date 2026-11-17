package io.nullwake.app.model

import org.json.JSONArray
import org.json.JSONObject
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

data class PeerNode(
    val id: String,
    val name: String,
    val ip: String,
    val online: Boolean,
    val connMode: String, // "P2P" | "DERP"
    val relay: String,
    val lastSeen: Long,
)

data class TelemetryMessage(
    val timestamp: Long,
    val data: TelemetryData,
)

data class TelemetryData(
    val cpu: CpuMetrics,
    val ram: RamMetrics,
    val net: NetMetrics,
    val gpu: GpuMetrics?,
    val services: List<ServiceStatus>,
)

data class CpuMetrics(
    val usagePercent: Double,
    val freqMHz: Int,
    val tempC: Double,
)

data class RamMetrics(
    val usedMB: Long,
    val totalMB: Long,
    val swapUsedMB: Long,
) {
    val usagePercent: Float
        get() = if (totalMB > 0L) ((usedMB.toDouble() / totalMB.toDouble()) * 100.0).toFloat().coerceIn(0f, 100f) else 0f
}

data class NetMetrics(
    val rxBytesSec: Long,
    val txBytesSec: Long,
)

data class GpuMetrics(
    val vendor: String, // "amd" | "intel"
    val name: String,
    val usagePercent: Double,
    val vramUsedMB: Long,
    val vramTotalMB: Long,
    val tempC: Double,
)

data class ServiceMetric(
    val label: String,
    val value: String,
)

data class ServiceStatus(
    val id: String,
    val name: String,
    val status: String, // "running" | "stopped" | "degraded"
    val summary: String,
    val metrics: List<ServiceMetric>,
    val history: List<HistoryEvent>,
)

data class HistoryEvent(
    val ts: Long,
    val event: String,
)

object TelemetryParser {
    fun parsePeerNodes(rawJson: String): List<PeerNode> {
        if (rawJson.isBlank()) return emptyList()
        return runCatching {
            val arr = JSONArray(rawJson)
            buildList(arr.length()) {
                for (i in 0 until arr.length()) {
                    val obj = arr.optJSONObject(i) ?: continue
                    add(
                        PeerNode(
                            id = obj.optString("id", ""),
                            name = obj.optString("name", ""),
                            ip = obj.optString("ip", ""),
                            online = obj.optBoolean("online", false),
                            connMode = obj.optString("conn_mode", "DERP"),
                            relay = obj.optString("relay", ""),
                            lastSeen = obj.optLong("last_seen", 0L),
                        ),
                    )
                }
            }
        }.getOrDefault(emptyList())
    }

    fun parseTelemetry(rawJson: String): TelemetryMessage? {
        if (rawJson.isBlank()) return null
        return runCatching {
            val root = JSONObject(rawJson)
            if (root.optString("type") != "TELEMETRY") return null
            val ts = root.optLong("timestamp", 0L)
            val data = root.optJSONObject("data") ?: return null

            val cpuObj = data.optJSONObject("cpu") ?: JSONObject()
            val ramObj = data.optJSONObject("ram") ?: JSONObject()
            val netObj = data.optJSONObject("net") ?: JSONObject()
            val gpuObj = if (data.isNull("gpu")) null else data.optJSONObject("gpu")

            val cpu = CpuMetrics(
                usagePercent = cpuObj.optDouble("usage_percent", 0.0),
                freqMHz = cpuObj.optInt("freq_mhz", 0),
                tempC = cpuObj.optDouble("temp_c", 0.0),
            )
            val ram = RamMetrics(
                usedMB = ramObj.optLong("used_mb", 0L),
                totalMB = ramObj.optLong("total_mb", 0L),
                swapUsedMB = ramObj.optLong("swap_used_mb", 0L),
            )
            val net = NetMetrics(
                rxBytesSec = netObj.optLong("rx_bytes_sec", 0L),
                txBytesSec = netObj.optLong("tx_bytes_sec", 0L),
            )
            val gpu = gpuObj?.let {
                GpuMetrics(
                    vendor = it.optString("vendor", "unknown"),
                    name = it.optString("name", "GPU"),
                    usagePercent = it.optDouble("usage_percent", 0.0),
                    vramUsedMB = it.optLong("vram_used_mb", 0L),
                    vramTotalMB = it.optLong("vram_total_mb", 0L),
                    tempC = it.optDouble("temp_c", 0.0),
                )
            }

            val servicesArr = data.optJSONArray("services") ?: JSONArray()
            val services = buildList(servicesArr.length()) {
                for (i in 0 until servicesArr.length()) {
                    val s = servicesArr.optJSONObject(i) ?: continue
                    val metricsList = buildList {
                        val metricsArr = s.optJSONArray("metrics")
                        if (metricsArr != null) {
                            for (m in 0 until metricsArr.length()) {
                                val metricObj = metricsArr.optJSONObject(m) ?: continue
                                val label = metricObj.optString("label", "").trim()
                                val value = metricObj.optString("value", "")
                                if (label.isNotEmpty()) {
                                    add(ServiceMetric(label = label, value = value))
                                }
                            }
                        } else {
                            // Legacy object fallback
                            val metricsObj = s.optJSONObject("metrics")
                            if (metricsObj != null) {
                                val keys = metricsObj.keys()
                                while (keys.hasNext()) {
                                    val k = keys.next()
                                    add(ServiceMetric(label = k, value = metricsObj.opt(k)?.toString() ?: ""))
                                }
                            }
                        }
                    }

                    val histArr = s.optJSONArray("history") ?: JSONArray()
                    val histList = buildList(histArr.length()) {
                        for (h in 0 until histArr.length()) {
                            val ev = histArr.optJSONObject(h) ?: continue
                            add(
                                HistoryEvent(
                                    ts = ev.optLong("ts", 0L),
                                    event = ev.optString("event", ""),
                                ),
                            )
                        }
                    }

                    add(
                        ServiceStatus(
                            id = s.optString("id", ""),
                            name = s.optString("name", ""),
                            status = s.optString("status", "stopped"),
                            summary = s.optString("summary", ""),
                            metrics = metricsList,
                            history = histList,
                        ),
                    )
                }
            }

            TelemetryMessage(
                timestamp = ts,
                data = TelemetryData(
                    cpu = cpu,
                    ram = ram,
                    net = net,
                    gpu = gpu,
                    services = services,
                ),
            )
        }.getOrNull()
    }
}

fun formatBytesPerSec(bytesPerSec: Long): String {
    if (bytesPerSec < 1024L) return "$bytesPerSec B/s"
    val kb = bytesPerSec / 1024.0
    if (kb < 1024.0) return String.format(Locale.US, "%.1f KB/s", kb)
    val mb = kb / 1024.0
    if (mb < 1024.0) return String.format(Locale.US, "%.2f MB/s", mb)
    val gb = mb / 1024.0
    return String.format(Locale.US, "%.2f GB/s", gb)
}

fun formatEpochSeconds(ts: Long): String {
    if (ts <= 0L) return "--:--:--"
    val sdf = SimpleDateFormat("MM-dd HH:mm:ss", Locale.getDefault())
    return sdf.format(Date(ts * 1000L))
}
