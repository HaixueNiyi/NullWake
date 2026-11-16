package io.nullwake.app.ui.screens

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.DeveloperBoard
import androidx.compose.material.icons.rounded.Memory
import androidx.compose.material.icons.rounded.NetworkCheck
import androidx.compose.material.icons.rounded.OpenInBrowser
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import io.nullwake.app.NullWakeUiState
import io.nullwake.app.model.GpuMetrics
import io.nullwake.app.model.formatBytesPerSec
import io.nullwake.app.ui.components.ServiceCardsSection
import io.nullwake.app.ui.components.SparklineCanvas
import io.nullwake.app.ui.theme.NullWakeColors
import java.util.Locale

@Composable
fun DashboardScreen(
    uiState: NullWakeUiState,
    onOpenAuthUrl: (String) -> Unit,
    onOpenSettings: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val telemetry = uiState.telemetry

    LazyColumn(
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        modifier = modifier.fillMaxSize(),
    ) {
        if (uiState.authUrl != null) {
            item {
                AuthRequiredCard(
                    authUrl = uiState.authUrl,
                    onOpenAuthUrl = onOpenAuthUrl,
                )
            }
        }

        if (!uiState.errorMessage.isNullOrBlank()) {
            item {
                Card(
                    colors = CardDefaults.cardColors(containerColor = NullWakeColors.StoppedRed.copy(alpha = 0.14f)),
                    border = BorderStroke(1.dp, NullWakeColors.StoppedRed.copy(alpha = 0.5f)),
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text(
                        text = uiState.errorMessage,
                        color = NullWakeColors.TextPrimary,
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.padding(12.dp),
                    )
                }
            }
        }

        if (telemetry == null) {
            item {
                EmptyWaitingCard(
                    tailscaleState = uiState.tailscaleState,
                    selectedHost = uiState.selectedNode?.name,
                    onOpenSettings = onOpenSettings,
                )
            }
        } else {
            val data = telemetry.data
            item {
                MetricSparklineCard(
                    title = "CPU",
                    icon = Icons.Rounded.DeveloperBoard,
                    headline = String.format(Locale.US, "%.1f%%", data.cpu.usagePercent),
                    subtitle = "${data.cpu.freqMHz} MHz  •  ${String.format(Locale.US, "%.1f°C", data.cpu.tempC)}",
                    accent = NullWakeColors.CpuAccent,
                    points = uiState.cpuHistory,
                    maxValue = 100f,
                )
            }

            item {
                MetricSparklineCard(
                    title = "Memory",
                    icon = Icons.Rounded.Memory,
                    headline = "${data.ram.usedMB} / ${data.ram.totalMB} MB",
                    subtitle = "Swap ${data.ram.swapUsedMB} MB  •  ${String.format(Locale.US, "%.1f%%", data.ram.usagePercent)}",
                    accent = NullWakeColors.RamAccent,
                    points = uiState.ramHistory,
                    maxValue = 100f,
                )
            }

            item {
                MetricSparklineCard(
                    title = "Network",
                    icon = Icons.Rounded.NetworkCheck,
                    headline = "↓ ${formatBytesPerSec(data.net.rxBytesSec)}   ↑ ${formatBytesPerSec(data.net.txBytesSec)}",
                    subtitle = "Violet: RX  •  Light Violet: TX  (60-sample window)",
                    accent = NullWakeColors.NetRxAccent,
                    points = uiState.netRxHistory,
                    secondaryPoints = uiState.netTxHistory,
                    secondaryAccent = NullWakeColors.NetTxAccent,
                    maxValue = null,
                )
            }

            // Conditionally render GPU card ONLY when the host reports a non-null GPU.
            if (data.gpu != null) {
                item {
                    GpuSparklineCard(
                        gpu = data.gpu,
                        points = uiState.gpuHistory,
                    )
                }
            }

            item {
                Spacer(modifier = Modifier.height(4.dp))
                ServiceCardsSection(services = data.services)
            }
        }
    }
}

@Composable
private fun AuthRequiredCard(
    authUrl: String,
    onOpenAuthUrl: (String) -> Unit,
) {
    Card(
        colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
        border = BorderStroke(1.dp, NullWakeColors.CpuAccent.copy(alpha = 0.6f)),
        shape = RoundedCornerShape(14.dp),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "Tailscale Authentication Required",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = NullWakeColors.TextPrimary,
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "Authenticate this device in your browser without creating an Android VPN profile.",
                style = MaterialTheme.typography.bodySmall,
                color = NullWakeColors.TextSecondary,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Button(onClick = { onOpenAuthUrl(authUrl) }) {
                Icon(
                    imageVector = Icons.Rounded.OpenInBrowser,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp),
                )
                Spacer(modifier = Modifier.size(8.dp))
                Text("Login to Tailscale")
            }
        }
    }
}

@Composable
private fun EmptyWaitingCard(
    tailscaleState: String,
    selectedHost: String?,
    onOpenSettings: () -> Unit,
) {
    Card(
        colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
        border = BorderStroke(1.dp, NullWakeColors.Border),
        shape = RoundedCornerShape(14.dp),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(modifier = Modifier.padding(18.dp)) {
            Text(
                text = if (selectedHost == null) "Waiting for Tailscale Host" else "Connecting to $selectedHost…",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = NullWakeColors.TextPrimary,
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = "Tailnet state: $tailscaleState. Zero telemetry is polled until you select an online host.",
                style = MaterialTheme.typography.bodySmall,
                color = NullWakeColors.TextSecondary,
            )
            AnimatedVisibility(visible = tailscaleState != "Running") {
                Column {
                    Spacer(modifier = Modifier.height(10.dp))
                    Button(onClick = onOpenSettings) {
                        Text("Open Connection Settings")
                    }
                }
            }
        }
    }
}

@Composable
private fun MetricSparklineCard(
    title: String,
    icon: ImageVector,
    headline: String,
    subtitle: String,
    accent: Color,
    points: List<Float>,
    maxValue: Float?,
    secondaryPoints: List<Float>? = null,
    secondaryAccent: Color = NullWakeColors.NetTxAccent,
) {
    Card(
        colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
        border = BorderStroke(1.dp, NullWakeColors.Border),
        shape = RoundedCornerShape(14.dp),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Icon(
                        imageVector = icon,
                        contentDescription = null,
                        tint = accent,
                        modifier = Modifier.size(18.dp),
                    )
                    Text(
                        text = title,
                        style = MaterialTheme.typography.labelLarge,
                        color = NullWakeColors.TextSecondary,
                    )
                }
                Text(
                    text = headline,
                    style = MaterialTheme.typography.titleMedium.copy(
                        fontFeatureSettings = "tnum",
                    ),
                    fontWeight = FontWeight.Bold,
                    fontFamily = FontFamily.Monospace,
                    color = NullWakeColors.TextPrimary,
                )
            }
            Spacer(modifier = Modifier.height(2.dp))
            Text(
                text = subtitle,
                style = MaterialTheme.typography.labelSmall.copy(
                    fontFeatureSettings = "tnum",
                ),
                color = NullWakeColors.TextSecondary,
            )
            Spacer(modifier = Modifier.height(10.dp))
            SparklineCanvas(
                points = points,
                lineColor = accent,
                maxValue = maxValue,
                secondaryPoints = secondaryPoints,
                secondaryColor = secondaryAccent,
            )
        }
    }
}

@Composable
private fun GpuSparklineCard(
    gpu: GpuMetrics,
    points: List<Float>,
) {
    val accent = NullWakeColors.GpuAccent

    Card(
        colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
        border = BorderStroke(1.dp, NullWakeColors.Border),
        shape = RoundedCornerShape(14.dp),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    modifier = Modifier.weight(1f),
                ) {
                    Surface(
                        color = accent.copy(alpha = 0.16f),
                        shape = RoundedCornerShape(6.dp),
                    ) {
                        Text(
                            text = gpu.vendor.uppercase(),
                            color = accent,
                            style = MaterialTheme.typography.labelSmall,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp),
                        )
                    }
                    Text(
                        text = gpu.name,
                        style = MaterialTheme.typography.labelLarge,
                        color = NullWakeColors.TextPrimary,
                    )
                }
                Text(
                    text = String.format(Locale.US, "%.1f%%", gpu.usagePercent),
                    style = MaterialTheme.typography.titleMedium.copy(
                        fontFeatureSettings = "tnum",
                    ),
                    fontWeight = FontWeight.Bold,
                    fontFamily = FontFamily.Monospace,
                    color = NullWakeColors.TextPrimary,
                )
            }
            Spacer(modifier = Modifier.height(4.dp))
            val vramPart = if (gpu.vramTotalMB > 0) {
                "VRAM ${gpu.vramUsedMB} / ${gpu.vramTotalMB} MB  •  "
            } else {
                ""
            }
            Text(
                text = "${vramPart}${String.format(Locale.US, "%.1f°C", gpu.tempC)}",
                style = MaterialTheme.typography.labelSmall.copy(
                    fontFeatureSettings = "tnum",
                ),
                color = NullWakeColors.TextSecondary,
            )
            Spacer(modifier = Modifier.height(10.dp))
            SparklineCanvas(
                points = points,
                lineColor = accent,
                maxValue = 100f,
            )
        }
    }
}
