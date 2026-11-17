package io.nullwake.app.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.History
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedCard
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.nullwake.app.model.ServiceStatus
import io.nullwake.app.model.formatEpochSeconds
import io.nullwake.app.ui.i18n.LocalAppStrings
import io.nullwake.app.ui.theme.LocalNullWakePalette

@Composable
fun ServiceCardsSection(services: List<ServiceStatus>) {
    val strings = LocalAppStrings.current
    val colors = MaterialTheme.colorScheme
    val palette = LocalNullWakePalette.current

    var inspectedServiceId by remember { mutableStateOf<String?>(null) }
    val inspectedService = services.firstOrNull { it.id == inspectedServiceId }

    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text(
            text = strings.servicesSectionTitle,
            style = MaterialTheme.typography.titleSmall,
            color = colors.onSurfaceVariant,
            fontWeight = FontWeight.SemiBold,
        )

        if (services.isEmpty()) {
            OutlinedCard(
                colors = CardDefaults.outlinedCardColors(containerColor = colors.surfaceContainer),
                border = BorderStroke(1.dp, palette.cardBorder),
                shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(
                    text = strings.noServicesConfigured,
                    style = MaterialTheme.typography.bodyMedium,
                    color = colors.onSurfaceVariant,
                    modifier = Modifier.padding(16.dp),
                )
            }
        } else {
            services.forEach { svc ->
                ServiceCard(
                    service = svc,
                    onClick = { inspectedServiceId = svc.id },
                )
            }
        }
    }

    if (inspectedService != null) {
        ServiceDetailBottomSheet(
            service = inspectedService,
            onDismiss = { inspectedServiceId = null },
        )
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ServiceCard(
    service: ServiceStatus,
    onClick: () -> Unit,
) {
    val strings = LocalAppStrings.current
    val colors = MaterialTheme.colorScheme

    ElevatedCard(
        onClick = onClick,
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
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = service.name,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                        color = colors.onSurface,
                    )
                    if (service.summary.isNotBlank()) {
                        Text(
                            text = service.summary,
                            style = MaterialTheme.typography.bodySmall,
                            color = colors.onSurfaceVariant,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                ServiceStatusPill(status = service.status)
            }

            if (service.metrics.isNotEmpty() || service.history.isNotEmpty()) {
                Spacer(modifier = Modifier.height(10.dp))
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    service.metrics.take(4).forEach { metric ->
                        Surface(
                            color = colors.surfaceContainerHigh,
                            shape = RoundedCornerShape(8.dp),
                        ) {
                            Text(
                                text = "${metric.label}: ${metric.value}",
                                style = MaterialTheme.typography.labelSmall.copy(
                                    fontFeatureSettings = "tnum",
                                ),
                                color = colors.onSurface,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                            )
                        }
                    }
                    if (service.history.isNotEmpty()) {
                        Surface(
                            color = colors.primaryContainer.copy(alpha = 0.7f),
                            shape = RoundedCornerShape(8.dp),
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                            ) {
                                Icon(
                                    imageVector = Icons.Rounded.History,
                                    contentDescription = null,
                                    tint = colors.onPrimaryContainer,
                                    modifier = Modifier.size(12.dp),
                                )
                                Text(
                                    text = " ${strings.eventsCount(service.history.size)}",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = colors.onPrimaryContainer,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun ServiceStatusPill(status: String) {
    val palette = LocalNullWakePalette.current
    val targetColor: Color = when (status.lowercase()) {
        "running" -> palette.onlineGreen
        "degraded" -> palette.degradedAmber
        else -> palette.stoppedRed
    }
    val color by animateColorAsState(
        targetValue = targetColor,
        animationSpec = tween(250),
        label = "service_status_color",
    )
    Surface(
        color = color.copy(alpha = 0.16f),
        shape = RoundedCornerShape(20.dp),
    ) {
        Text(
            text = status.uppercase(),
            color = color,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ServiceDetailBottomSheet(
    service: ServiceStatus,
    onDismiss: () -> Unit,
) {
    val strings = LocalAppStrings.current
    val colors = MaterialTheme.colorScheme
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = colors.surfaceContainer,
        contentColor = colors.onSurface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 8.dp),
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = service.name,
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                    )
                    Text(
                        text = "id: ${service.id} • ${service.summary}",
                        style = MaterialTheme.typography.bodySmall,
                        color = colors.onSurfaceVariant,
                    )
                }
                ServiceStatusPill(status = service.status)
            }

            if (service.metrics.isNotEmpty()) {
                Spacer(modifier = Modifier.height(14.dp))
                Text(
                    text = strings.customMetrics,
                    style = MaterialTheme.typography.labelLarge,
                    color = colors.onSurfaceVariant,
                )
                Spacer(modifier = Modifier.height(6.dp))
                service.metrics.forEach { metric ->
                    Row(
                        horizontalArrangement = Arrangement.SpaceBetween,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 3.dp),
                    ) {
                        Text(text = metric.label, color = colors.onSurfaceVariant, style = MaterialTheme.typography.bodySmall)
                        Text(
                            text = metric.value,
                            color = colors.onSurface,
                            style = MaterialTheme.typography.bodySmall.copy(
                                fontFeatureSettings = "tnum",
                            ),
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(14.dp))
            HorizontalDivider(color = colors.outlineVariant)
            Spacer(modifier = Modifier.height(12.dp))

            Text(
                text = strings.ringBufferHistory(service.history.size),
                style = MaterialTheme.typography.labelLarge,
                color = colors.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.height(8.dp))

            val newestFirst = remember(service.history) { service.history.asReversed() }
            if (newestFirst.isEmpty()) {
                Text(
                    text = strings.noPushEventsYet,
                    style = MaterialTheme.typography.bodySmall,
                    color = colors.onSurfaceVariant,
                    modifier = Modifier.padding(bottom = 24.dp),
                )
            } else {
                LazyColumn(
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(260.dp)
                        .padding(bottom = 16.dp),
                ) {
                    items(newestFirst) { item ->
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Text(
                                text = formatEpochSeconds(item.ts),
                                style = MaterialTheme.typography.labelSmall.copy(
                                    fontFeatureSettings = "tnum",
                                ),
                                fontFamily = FontFamily.Monospace,
                                color = colors.primary,
                            )
                            Text(
                                text = item.event,
                                style = MaterialTheme.typography.bodySmall,
                                color = colors.onSurface,
                                modifier = Modifier.weight(1f),
                            )
                        }
                    }
                }
            }
        }
    }
}
