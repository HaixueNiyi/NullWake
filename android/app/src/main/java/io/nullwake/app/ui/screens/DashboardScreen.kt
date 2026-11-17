package io.nullwake.app.ui.screens

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.DeveloperBoard
import androidx.compose.material.icons.rounded.Memory
import androidx.compose.material.icons.rounded.NetworkCheck
import androidx.compose.material.icons.rounded.OpenInBrowser
import androidx.compose.material3.Button
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedCard
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import io.nullwake.app.NullWakeUiState
import io.nullwake.app.model.GpuMetrics
import io.nullwake.app.model.formatBytesPerSec
import io.nullwake.app.ui.components.ServiceCardsSection
import io.nullwake.app.ui.components.SparklineCanvas
import io.nullwake.app.ui.i18n.LocalAppStrings
import io.nullwake.app.ui.theme.LocalNullWakePalette
import java.util.Locale

@Composable
fun DashboardScreen(
    uiState: NullWakeUiState,
    onOpenAuthUrl: (String) -> Unit,
    onOpenSettings: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val strings = LocalAppStrings.current
    val palette = LocalNullWakePalette.current
    val colors = MaterialTheme.colorScheme
    val telemetry = uiState.telemetry

    LazyColumn(
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        modifier = modifier.fillMaxSize(),
    ) {
        if (uiState.authUrl != null) {
            item(key = "auth_banner") {
                AuthRequiredCard(
                    authUrl = uiState.authUrl,
                    onOpenAuthUrl = onOpenAuthUrl,
                )
            }
        }

        if (!uiState.errorMessage.isNullOrBlank()) {
            item(key = "error_banner") {
                OutlinedCard(
                    colors = CardDefaults.outlinedCardColors(
                        containerColor = palette.stoppedRed.copy(alpha = 0.12f),
                    ),
                    border = BorderStroke(1.dp, palette.stoppedRed.copy(alpha = 0.45f)),
                    shape = MaterialTheme.shapes.medium,
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text(
                        text = uiState.errorMessage,
                        color = colors.onSurface,
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.padding(14.dp),
                    )
                }
            }
        }

        if (telemetry == null) {
            item {
                val selectedDisplayName = uiState.selectedNode?.let { uiState.displayNameFor(it) }
                EmptyWaitingCard(
                    tailscaleState = uiState.tailscaleState,
                    selectedHost = selectedDisplayName,
                    isConnecting = uiState.isConnecting,
                    onOpenSettings = onOpenSettings,
                )
            }
        } else {
            val data = telemetry.data

            item {
                MetricSparklineCard(
                    title = strings.cpuTitle,
                    icon = Icons.Rounded.DeveloperBoard,
                    headline = AnnotatedString(String.format(Locale.US, "%.1f%%", data.cpu.usagePercent)),
                    subtitle = AnnotatedString("${data.cpu.freqMHz} MHz  •  ${String.format(Locale.US, "%.1f°C", data.cpu.tempC)}"),
                    accent = palette.cpuAccent,
                    points = uiState.cpuHistory,
                    maxValue = 100f,
                )
            }

            item {
                MetricSparklineCard(
                    title = strings.memoryTitle,
                    icon = Icons.Rounded.Memory,
                    headline = AnnotatedString("${data.ram.usedMB} / ${data.ram.totalMB} MB"),
                    subtitle = AnnotatedString("${strings.swapLabel} ${data.ram.swapUsedMB} MB  •  ${String.format(Locale.US, "%.1f%%", data.ram.usagePercent)}"),
                    accent = palette.ramAccent,
                    points = uiState.ramHistory,
                    maxValue = 100f,
                )
            }

            item {
                val rxFormatted = formatBytesPerSec(data.net.rxBytesSec)
                val txFormatted = formatBytesPerSec(data.net.txBytesSec)
                val netHeadline = buildAnnotatedString {
                    withStyle(SpanStyle(color = palette.netRxAccent)) {
                        append("↓ $rxFormatted")
                    }
                    append("   ")
                    withStyle(SpanStyle(color = palette.netTxAccent)) {
                        append("↑ $txFormatted")
                    }
                }
                val netSubtitle = buildAnnotatedString {
                    withStyle(SpanStyle(color = palette.netRxAccent, fontWeight = FontWeight.SemiBold)) {
                        append("● ${strings.rxLabel}")
                    }
                    append("  •  ")
                    withStyle(SpanStyle(color = palette.netTxAccent, fontWeight = FontWeight.SemiBold)) {
                        append("● ${strings.txLabel}")
                    }
                    append("  (${strings.window60Samples})")
                }

                MetricSparklineCard(
                    title = strings.networkTitle,
                    icon = Icons.Rounded.NetworkCheck,
                    headline = netHeadline,
                    subtitle = netSubtitle,
                    accent = palette.netRxAccent,
                    points = uiState.netRxHistory,
                    secondaryPoints = uiState.netTxHistory,
                    secondaryAccent = palette.netTxAccent,
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
    val strings = LocalAppStrings.current
    val colors = MaterialTheme.colorScheme

    ElevatedCard(
        colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainerHigh),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(
            modifier = Modifier
                .animateContentSize(animationSpec = tween(220))
                .padding(16.dp),
        ) {
            Text(
                text = strings.authRequiredTitle,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = colors.onSurface,
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = strings.authRequiredBody,
                style = MaterialTheme.typography.bodySmall,
                color = colors.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Button(onClick = { onOpenAuthUrl(authUrl) }) {
                Icon(
                    imageVector = Icons.Rounded.OpenInBrowser,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp),
                )
                Spacer(modifier = Modifier.size(8.dp))
                Text(strings.loginToTailscale)
            }
        }
    }
}

@Composable
private fun EmptyWaitingCard(
    tailscaleState: String,
    selectedHost: String?,
    isConnecting: Boolean,
    onOpenSettings: () -> Unit,
) {
    val strings = LocalAppStrings.current
    val colors = MaterialTheme.colorScheme
    val palette = LocalNullWakePalette.current

    OutlinedCard(
        colors = CardDefaults.outlinedCardColors(containerColor = colors.surfaceContainer),
        border = BorderStroke(1.dp, palette.cardBorder),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(
            modifier = Modifier
                .animateContentSize(animationSpec = tween(220))
                .padding(18.dp),
        ) {
            Text(
                text = if (selectedHost == null) strings.waitingForHost else strings.connectingTo(selectedHost),
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = colors.onSurface,
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = if (isConnecting) strings.connectingProgress else strings.tailnetStateDesc(tailscaleState),
                style = MaterialTheme.typography.bodySmall,
                color = colors.onSurfaceVariant,
            )
            if (isConnecting || tailscaleState == "Starting") {
                Spacer(modifier = Modifier.height(12.dp))
                LinearProgressIndicator(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(4.dp)),
                )
            }
            AnimatedVisibility(visible = tailscaleState != "Running") {
                Column {
                    Spacer(modifier = Modifier.height(12.dp))
                    FilledTonalButton(onClick = onOpenSettings) {
                        Text(strings.openConnectionSettings)
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
    headline: AnnotatedString,
    subtitle: AnnotatedString,
    accent: Color,
    points: List<Float>,
    maxValue: Float?,
    secondaryPoints: List<Float>? = null,
    secondaryAccent: Color = accent,
) {
    val colors = MaterialTheme.colorScheme
    val palette = LocalNullWakePalette.current

    ElevatedCard(
        colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainer),
        elevation = CardDefaults.elevatedCardElevation(defaultElevation = 1.dp),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(
            modifier = Modifier
                .animateContentSize(animationSpec = tween(220))
                .padding(16.dp),
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Box(
                        contentAlignment = Alignment.Center,
                        modifier = Modifier
                            .size(28.dp)
                            .clip(CircleShape)
                            .background(accent.copy(alpha = 0.14f)),
                    ) {
                        Icon(
                            imageVector = icon,
                            contentDescription = null,
                            tint = accent,
                            modifier = Modifier.size(16.dp),
                        )
                    }
                    Text(
                        text = title,
                        style = MaterialTheme.typography.titleSmall,
                        color = colors.onSurfaceVariant,
                        fontWeight = FontWeight.Medium,
                    )
                }
                Text(
                    text = headline,
                    style = MaterialTheme.typography.titleMedium.copy(
                        fontFeatureSettings = "tnum",
                    ),
                    fontWeight = FontWeight.Bold,
                    fontFamily = FontFamily.Monospace,
                    color = colors.onSurface,
                )
            }
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = subtitle,
                style = MaterialTheme.typography.labelSmall.copy(
                    fontFeatureSettings = "tnum",
                ),
                color = colors.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.height(10.dp))
            Surface(
                color = colors.surfaceContainerLow,
                shape = MaterialTheme.shapes.small,
                border = BorderStroke(1.dp, palette.cardBorder.copy(alpha = 0.6f)),
            ) {
                SparklineCanvas(
                    points = points,
                    lineColor = accent,
                    maxValue = maxValue,
                    secondaryPoints = secondaryPoints,
                    secondaryColor = secondaryAccent,
                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 4.dp),
                )
            }
        }
    }
}

@Composable
private fun GpuSparklineCard(
    gpu: GpuMetrics,
    points: List<Float>,
) {
    val strings = LocalAppStrings.current
    val colors = MaterialTheme.colorScheme
    val palette = LocalNullWakePalette.current
    val accent = palette.gpuAccent

    ElevatedCard(
        colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainer),
        elevation = CardDefaults.elevatedCardElevation(defaultElevation = 1.dp),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(
            modifier = Modifier
                .animateContentSize(animationSpec = tween(220))
                .padding(16.dp),
        ) {
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
                        shape = RoundedCornerShape(8.dp),
                    ) {
                        Text(
                            text = gpu.vendor.uppercase(),
                            color = accent,
                            style = MaterialTheme.typography.labelSmall,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
                        )
                    }
                    Text(
                        text = gpu.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = colors.onSurface,
                        fontWeight = FontWeight.Medium,
                    )
                }
                Text(
                    text = String.format(Locale.US, "%.1f%%", gpu.usagePercent),
                    style = MaterialTheme.typography.titleMedium.copy(
                        fontFeatureSettings = "tnum",
                    ),
                    fontWeight = FontWeight.Bold,
                    fontFamily = FontFamily.Monospace,
                    color = colors.onSurface,
                )
            }
            Spacer(modifier = Modifier.height(6.dp))
            val vramPart = if (gpu.vramTotalMB > 0) {
                "${strings.vramLabel} ${gpu.vramUsedMB} / ${gpu.vramTotalMB} MB  •  "
            } else {
                ""
            }
            Text(
                text = "${vramPart}${String.format(Locale.US, "%.1f°C", gpu.tempC)}",
                style = MaterialTheme.typography.labelSmall.copy(
                    fontFeatureSettings = "tnum",
                ),
                color = colors.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.height(10.dp))
            Surface(
                color = colors.surfaceContainerLow,
                shape = MaterialTheme.shapes.small,
                border = BorderStroke(1.dp, palette.cardBorder.copy(alpha = 0.6f)),
            ) {
                SparklineCanvas(
                    points = points,
                    lineColor = accent,
                    maxValue = 100f,
                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 4.dp),
                )
            }
        }
    }
}
