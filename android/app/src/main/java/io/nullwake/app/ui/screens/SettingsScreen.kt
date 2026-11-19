package io.nullwake.app.ui.screens

import android.os.Build
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.isSystemInDarkTheme
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Edit
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import io.nullwake.app.NullWakeUiState
import io.nullwake.app.model.PeerNode
import io.nullwake.app.ui.components.RenameDeviceDialog
import io.nullwake.app.ui.i18n.AppLanguage
import io.nullwake.app.ui.i18n.LocalAppStrings
import io.nullwake.app.ui.theme.LocalNullWakePalette
import io.nullwake.app.ui.theme.ThemeMode

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    uiState: NullWakeUiState,
    onSelectThemeMode: (ThemeMode) -> Unit,
    onToggleAmoled: (Boolean) -> Unit,
    onToggleDynamicColor: (Boolean) -> Unit,
    onSelectLanguage: (AppLanguage) -> Unit,
    onRenameNode: (nodeId: String, newName: String) -> Unit,
    onSelectInterval: (Long) -> Unit,
    onApplyConnectionSettings: (proxyUrl: String, authKey: String, agentPort: String) -> Unit,
    onOpenAuthUrl: (String) -> Unit,
    onResetLogin: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val strings = LocalAppStrings.current
    val colors = MaterialTheme.colorScheme
    val palette = LocalNullWakePalette.current

    var proxyInput by remember(uiState.proxyUrl) { mutableStateOf(uiState.proxyUrl) }
    var authKeyInput by remember(uiState.authKey) { mutableStateOf(uiState.authKey) }
    var agentPortInput by remember(uiState.agentPort) { mutableStateOf(uiState.agentPort.toString()) }
    var editingNode by remember { mutableStateOf<PeerNode?>(null) }

    val intervals = listOf(500L, 1000L, 2000L)
    val themeModes = listOf(ThemeMode.SYSTEM, ThemeMode.LIGHT, ThemeMode.DARK)
    val languages = listOf(AppLanguage.SYSTEM, AppLanguage.ZH_TW, AppLanguage.EN)
    val supportsDynamicColor = Build.VERSION.SDK_INT >= Build.VERSION_CODES.S
    val isDarkEffective = when (uiState.themeMode) {
        ThemeMode.SYSTEM -> isSystemInDarkTheme()
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
    }

    LazyColumn(
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
        modifier = modifier.fillMaxSize(),
    ) {
        // Appearance & Theme Card
        item {
            ElevatedCard(
                colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainer),
                shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = strings.appearanceSectionTitle,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(10.dp))
                    Text(
                        text = strings.themeModeLabel,
                        style = MaterialTheme.typography.labelMedium,
                        color = colors.onSurfaceVariant,
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
                        themeModes.forEachIndexed { index, mode ->
                            val label = when (mode) {
                                ThemeMode.SYSTEM -> strings.themeSystem
                                ThemeMode.LIGHT -> strings.themeLight
                                ThemeMode.DARK -> strings.themeDark
                            }
                            SegmentedButton(
                                selected = uiState.themeMode == mode,
                                onClick = { onSelectThemeMode(mode) },
                                shape = SegmentedButtonDefaults.itemShape(index = index, count = themeModes.size),
                                label = { Text(label) },
                            )
                        }
                    }

                    Spacer(modifier = Modifier.height(14.dp))
                    HorizontalDivider(color = colors.outlineVariant)
                    Spacer(modifier = Modifier.height(10.dp))

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Column(modifier = Modifier.weight(1f).padding(end = 12.dp)) {
                            Text(
                                text = strings.amoledBlackTitle,
                                style = MaterialTheme.typography.bodyMedium,
                                fontWeight = FontWeight.Medium,
                            )
                            Text(
                                text = strings.amoledBlackSubtitle,
                                style = MaterialTheme.typography.bodySmall,
                                color = colors.onSurfaceVariant,
                            )
                        }
                        Switch(
                            checked = uiState.amoledBlack,
                            onCheckedChange = onToggleAmoled,
                            enabled = isDarkEffective,
                        )
                    }

                    Spacer(modifier = Modifier.height(10.dp))
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Column(modifier = Modifier.weight(1f).padding(end = 12.dp)) {
                            Text(
                                text = strings.dynamicColorTitle,
                                style = MaterialTheme.typography.bodyMedium,
                                fontWeight = FontWeight.Medium,
                            )
                            Text(
                                text = if (supportsDynamicColor) strings.dynamicColorSubtitle else strings.dynamicColorUnsupported,
                                style = MaterialTheme.typography.bodySmall,
                                color = colors.onSurfaceVariant,
                            )
                        }
                        Switch(
                            checked = uiState.dynamicColor && supportsDynamicColor,
                            onCheckedChange = onToggleDynamicColor,
                            enabled = supportsDynamicColor,
                        )
                    }

                    Spacer(modifier = Modifier.height(14.dp))
                    HorizontalDivider(color = colors.outlineVariant)
                    Spacer(modifier = Modifier.height(10.dp))

                    Text(
                        text = strings.languageTitle,
                        style = MaterialTheme.typography.labelMedium,
                        color = colors.onSurfaceVariant,
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
                        languages.forEachIndexed { index, lang ->
                            val label = when (lang) {
                                AppLanguage.SYSTEM -> strings.langSystem
                                AppLanguage.ZH_TW -> strings.langZhTw
                                AppLanguage.EN -> strings.langEn
                            }
                            SegmentedButton(
                                selected = uiState.language == lang,
                                onClick = { onSelectLanguage(lang) },
                                shape = SegmentedButtonDefaults.itemShape(index = index, count = languages.size),
                                label = { Text(label) },
                            )
                        }
                    }
                }
            }
        }

        // Custom Device Names Card
        item {
            ElevatedCard(
                colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainer),
                shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = strings.customDeviceNamesTitle,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = strings.customDeviceNamesDesc,
                        style = MaterialTheme.typography.bodySmall,
                        color = colors.onSurfaceVariant,
                    )
                    Spacer(modifier = Modifier.height(10.dp))
                    if (uiState.nodes.isEmpty()) {
                        Text(
                            text = strings.noNodesToRename,
                            style = MaterialTheme.typography.bodySmall,
                            color = colors.onSurfaceVariant,
                        )
                    } else {
                        uiState.nodes.forEachIndexed { idx, node ->
                            if (idx > 0) {
                                HorizontalDivider(color = colors.outlineVariant.copy(alpha = 0.6f))
                            }
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.SpaceBetween,
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(vertical = 6.dp),
                            ) {
                                Column(modifier = Modifier.weight(1f)) {
                                    Text(
                                        text = uiState.displayNameFor(node),
                                        style = MaterialTheme.typography.bodyMedium,
                                        fontWeight = FontWeight.Medium,
                                    )
                                    Text(
                                        text = "${node.name} • ${node.ip}",
                                        style = MaterialTheme.typography.labelSmall,
                                        color = colors.onSurfaceVariant,
                                    )
                                }
                                IconButton(onClick = { editingNode = node }) {
                                    Icon(
                                        imageVector = Icons.Rounded.Edit,
                                        contentDescription = strings.renameDevice,
                                        tint = colors.primary,
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }

        // Polling Interval Card
        item {
            ElevatedCard(
                colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainer),
                shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = strings.pollingIntervalTitle,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = strings.pollingIntervalDesc,
                        style = MaterialTheme.typography.bodySmall,
                        color = colors.onSurfaceVariant,
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

        // Tailscale Control & Proxy Card
        item {
            ElevatedCard(
                colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainer),
                shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = strings.tailscaleSectionTitle,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = strings.tailscaleStateHint(uiState.tailscaleState),
                        style = MaterialTheme.typography.bodySmall,
                        color = colors.primary,
                    )
                    Spacer(modifier = Modifier.height(12.dp))

                    OutlinedTextField(
                        value = proxyInput,
                        onValueChange = { proxyInput = it },
                        label = { Text(strings.proxyLabel) },
                        placeholder = { Text("socks5://127.0.0.1:10808") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = strings.proxyHint,
                        style = MaterialTheme.typography.labelSmall,
                        color = colors.onSurfaceVariant,
                    )

                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = authKeyInput,
                        onValueChange = { authKeyInput = it },
                        label = { Text(strings.authKeyLabel) },
                        placeholder = { Text("tskey-auth-...") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )

                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = agentPortInput,
                        onValueChange = { next ->
                            if (next.length <= 5 && next.all { it.isDigit() }) {
                                agentPortInput = next
                            }
                        },
                        label = { Text(strings.agentPortLabel) },
                        placeholder = { Text("9100") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = strings.agentPortHint,
                        style = MaterialTheme.typography.labelSmall,
                        color = colors.onSurfaceVariant,
                    )

                    Spacer(modifier = Modifier.height(14.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Button(onClick = { onApplyConnectionSettings(proxyInput, authKeyInput, agentPortInput) }) {
                            Text(strings.applyAndReconnect)
                        }
                        if (uiState.authUrl != null) {
                            OutlinedButton(onClick = { onOpenAuthUrl(uiState.authUrl) }) {
                                Text(strings.openLoginUrl)
                            }
                        }
                    }
                }
            }
        }

        // Node Identity Card
        item {
            ElevatedCard(
                colors = CardDefaults.elevatedCardColors(containerColor = colors.surfaceContainer),
                shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = strings.nodeIdentityTitle,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = strings.nodeIdentityDesc,
                        style = MaterialTheme.typography.bodySmall,
                        color = colors.onSurfaceVariant,
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedButton(
                        onClick = onResetLogin,
                        colors = ButtonDefaults.outlinedButtonColors(contentColor = palette.stoppedRed),
                        border = BorderStroke(1.dp, palette.stoppedRed.copy(alpha = 0.6f)),
                    ) {
                        Text(strings.resetTailscaleLogin)
                    }
                }
            }
        }
    }

    editingNode?.let { target ->
        RenameDeviceDialog(
            node = target,
            currentCustomName = uiState.customNames[target.id] ?: "",
            onDismiss = { editingNode = null },
            onSave = { newName ->
                onRenameNode(target.id, newName)
                editingNode = null
            },
        )
    }
}
