package io.nullwake.app.ui.screens

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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import io.nullwake.app.NullWakeUiState
import io.nullwake.app.ui.theme.NullWakeColors

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    uiState: NullWakeUiState,
    onSelectInterval: (Long) -> Unit,
    onApplyConnectionSettings: (proxyUrl: String, authKey: String) -> Unit,
    onOpenAuthUrl: (String) -> Unit,
    onResetLogin: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var proxyInput by remember(uiState.proxyUrl) { mutableStateOf(uiState.proxyUrl) }
    var authKeyInput by remember(uiState.authKey) { mutableStateOf(uiState.authKey) }

    val intervals = listOf(500L, 1000L, 2000L)

    LazyColumn(
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
        modifier = modifier.fillMaxSize(),
    ) {
        item {
            Card(
                colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
                border = BorderStroke(1.dp, NullWakeColors.Border),
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "Telemetry Polling Interval",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "Requested via WebSocket SUBSCRIBE. Collectors pause immediately when backgrounded.",
                        style = MaterialTheme.typography.bodySmall,
                        color = NullWakeColors.TextSecondary,
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
                        intervals.forEachIndexed { index, ms ->
                            SegmentedButton(
                                selected = uiState.intervalMs == ms,
                                onClick = { onSelectInterval(ms) },
                                shape = SegmentedButtonDefaults.itemShape(index = index, count = intervals.size),
                                label = { Text("${ms}ms") },
                            )
                        }
                    }
                }
            }
        }

        item {
            Card(
                colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
                border = BorderStroke(1.dp, NullWakeColors.Border),
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "Tailscale Control & Proxy (Optional)",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "Current state: ${uiState.tailscaleState}. Both fields below are optional — leave them blank for normal browser login.",
                        style = MaterialTheme.typography.bodySmall,
                        color = NullWakeColors.CpuAccent,
                    )
                    Spacer(modifier = Modifier.height(12.dp))

                    OutlinedTextField(
                        value = proxyInput,
                        onValueChange = { proxyInput = it },
                        label = { Text("Local Proxy URL (Optional — leave blank if unused)") },
                        placeholder = { Text("socks5://127.0.0.1:10808") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "Only needed if you run a local proxy app (e.g. Clash/v2ray/NekoBox) on this phone. Routes control-plane & DERP TCP handshakes while WireGuard UDP P2P stays direct.",
                        style = MaterialTheme.typography.labelSmall,
                        color = NullWakeColors.TextSecondary,
                    )

                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = authKeyInput,
                        onValueChange = { authKeyInput = it },
                        label = { Text("Pre-Auth Key (Optional — leave blank for browser login)") },
                        placeholder = { Text("tskey-auth-...") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )

                    Spacer(modifier = Modifier.height(14.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Button(onClick = { onApplyConnectionSettings(proxyInput, authKeyInput) }) {
                            Text("Apply & Reconnect")
                        }
                        if (uiState.authUrl != null) {
                            OutlinedButton(onClick = { onOpenAuthUrl(uiState.authUrl) }) {
                                Text("Open Login URL")
                            }
                        }
                    }
                }
            }
        }

        item {
            Card(
                colors = CardDefaults.cardColors(containerColor = NullWakeColors.Surface),
                border = BorderStroke(1.dp, NullWakeColors.Border),
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "Node Identity",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "Clears local tsnet state and initiates a fresh browser login flow.",
                        style = MaterialTheme.typography.bodySmall,
                        color = NullWakeColors.TextSecondary,
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedButton(
                        onClick = onResetLogin,
                        colors = ButtonDefaults.outlinedButtonColors(contentColor = NullWakeColors.StoppedRed),
                        border = BorderStroke(1.dp, NullWakeColors.StoppedRed.copy(alpha = 0.6f)),
                    ) {
                        Text("Reset Tailscale Login")
                    }
                }
            }
        }
    }
}
