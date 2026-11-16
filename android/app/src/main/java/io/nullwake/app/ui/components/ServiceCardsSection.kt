package io.nullwake.app.ui.components

import androidx.compose.foundation.BorderStroke
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
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
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
import io.nullwake.app.ui.theme.NullWakeColors

@Composable
fun ServiceCardsSection(services: List<ServiceStatus>) {
    var inspectedServiceId by remember { mutableStateOf<String?>(null) }
    val inspectedService = services.firstOrNull { it.id == inspectedServiceId }

    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text(
            text = "Services & Event Ring Buffer",
            style = MaterialTheme.typography.titleSmall,
            color = NullWakeColors.TextSecondary,
            fontWeight = FontWeight.SemiBold,
        )

        if (services.isEmpty()) {
            Card(
                colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
                border = BorderStroke(1.dp, NullWakeColors.Border),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(
                    text = "No services configured on this host.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = NullWakeColors.TextSecondary,
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
    Card(
        onClick = onClick,
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
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = service.name,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                        color = NullWakeColors.TextPrimary,
                    )
                    if (service.summary.isNotBlank()) {
                        Text(
                            text = service.summary,
                            style = MaterialTheme.typography.bodySmall,
                            color = NullWakeColors.TextSecondary,
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
                    service.metrics.entries.take(4).forEach { (k, v) ->
                        Surface(
                            color = NullWakeColors.SurfaceVariant,
                            shape = RoundedCornerShape(6.dp),
                        ) {
                            Text(
                                text = "$k: $v",
                                style = MaterialTheme.typography.labelSmall,
                                color = NullWakeColors.TextPrimary,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                            )
                        }
                    }
                    if (service.history.isNotEmpty()) {
                        Surface(
                            color = NullWakeColors.CpuAccent.copy(alpha = 0.14f),
                            shape = RoundedCornerShape(6.dp),
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                            ) {
                                Icon(
                                    imageVector = Icons.Rounded.History,
                                    contentDescription = null,
                                    tint = NullWakeColors.CpuAccent,
                                    modifier = Modifier.size(12.dp),
                                )
                                Text(
                                    text = " ${service.history.size} events",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = NullWakeColors.CpuAccent,
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
    val color: Color = when (status.lowercase()) {
        "running" -> NullWakeColors.OnlineGreen
        "degraded" -> NullWakeColors.DegradedAmber
        else -> NullWakeColors.StoppedRed
    }
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
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = NullWakeColors.Surface,
        contentColor = NullWakeColors.TextPrimary,
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
                        color = NullWakeColors.TextSecondary,
                    )
                }
                ServiceStatusPill(status = service.status)
            }

            if (service.metrics.isNotEmpty()) {
                Spacer(modifier = Modifier.height(14.dp))
                Text(
                    text = "Custom Metrics",
                    style = MaterialTheme.typography.labelLarge,
                    color = NullWakeColors.TextSecondary,
                )
                Spacer(modifier = Modifier.height(6.dp))
                service.metrics.forEach { (k, v) ->
                    Row(
                        horizontalArrangement = Arrangement.SpaceBetween,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 3.dp),
                    ) {
                        Text(text = k, color = NullWakeColors.TextSecondary, style = MaterialTheme.typography.bodySmall)
                        Text(
                            text = v,
                            color = NullWakeColors.TextPrimary,
                            style = MaterialTheme.typography.bodySmall,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(14.dp))
            HorizontalDivider(color = NullWakeColors.Border)
            Spacer(modifier = Modifier.height(12.dp))

            Text(
                text = "In-Memory Ring Buffer History (${service.history.size})",
                style = MaterialTheme.typography.labelLarge,
                color = NullWakeColors.TextSecondary,
            )
            Spacer(modifier = Modifier.height(8.dp))

            val newestFirst = remember(service.history) { service.history.asReversed() }
            if (newestFirst.isEmpty()) {
                Text(
                    text = "No push events recorded yet.",
                    style = MaterialTheme.typography.bodySmall,
                    color = NullWakeColors.TextSecondary,
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
                                style = MaterialTheme.typography.labelSmall,
                                fontFamily = FontFamily.Monospace,
                                color = NullWakeColors.CpuAccent,
                            )
                            Text(
                                text = item.event,
                                style = MaterialTheme.typography.bodySmall,
                                color = NullWakeColors.TextPrimary,
                                modifier = Modifier.weight(1f),
                            )
                        }
                    }
                }
            }
        }
    }
}
