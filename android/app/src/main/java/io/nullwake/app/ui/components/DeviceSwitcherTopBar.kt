package io.nullwake.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.Edit
import androidx.compose.material.icons.rounded.KeyboardArrowDown
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.nullwake.app.model.PeerNode
import io.nullwake.app.ui.i18n.LocalAppStrings
import io.nullwake.app.ui.theme.LocalNullWakePalette

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DeviceSwitcherTopBar(
    nodes: List<PeerNode>,
    selectedNode: PeerNode?,
    customNames: Map<String, String>,
    isSettingsOpen: Boolean,
    onSelectNode: (PeerNode) -> Unit,
    onRenameNode: (nodeId: String, newName: String) -> Unit,
    onRefreshNodes: () -> Unit,
    onToggleSettings: () -> Unit,
) {
    val strings = LocalAppStrings.current
    val palette = LocalNullWakePalette.current
    val colors = MaterialTheme.colorScheme

    var expanded by remember { mutableStateOf(false) }
    var editingNode by remember { mutableStateOf<PeerNode?>(null) }

    TopAppBar(
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = colors.background,
            titleContentColor = colors.onBackground,
        ),
        navigationIcon = {
            if (isSettingsOpen) {
                IconButton(onClick = onToggleSettings) {
                    Icon(
                        imageVector = Icons.AutoMirrored.Rounded.ArrowBack,
                        contentDescription = strings.backContentDesc,
                        tint = colors.onBackground,
                    )
                }
            }
        },
        title = {
            if (isSettingsOpen) {
                Text(
                    text = strings.settingsTitle,
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.SemiBold,
                )
            } else {
                Box {
                    Surface(
                        color = colors.surfaceContainerHigh,
                        shape = RoundedCornerShape(14.dp),
                        tonalElevation = 2.dp,
                        modifier = Modifier
                            .clip(RoundedCornerShape(14.dp))
                            .clickable {
                                onRefreshNodes()
                                expanded = true
                            },
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                        ) {
                            val dotColor = when {
                                selectedNode == null -> colors.onSurfaceVariant
                                selectedNode.online -> palette.onlineGreen
                                else -> palette.stoppedRed
                            }
                            Box(
                                modifier = Modifier
                                    .size(9.dp)
                                    .clip(CircleShape)
                                    .background(dotColor),
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            val activeDisplay = selectedNode?.let { node ->
                                customNames[node.id]?.takeIf { it.isNotBlank() } ?: node.name
                            }
                            Text(
                                text = activeDisplay ?: strings.selectHost,
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.SemiBold,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            if (selectedNode != null) {
                                Spacer(modifier = Modifier.width(8.dp))
                                ConnModeBadge(mode = selectedNode.connMode)
                            }
                            Spacer(modifier = Modifier.width(4.dp))
                            Icon(
                                imageVector = Icons.Rounded.KeyboardArrowDown,
                                contentDescription = strings.selectHost,
                                modifier = Modifier.size(18.dp),
                                tint = colors.onSurfaceVariant,
                            )
                        }
                    }

                    DropdownMenu(
                        expanded = expanded,
                        onDismissRequest = { expanded = false },
                        containerColor = colors.surfaceContainerHigh,
                    ) {
                        if (nodes.isEmpty()) {
                            DropdownMenuItem(
                                text = {
                                    Text(
                                        text = strings.noPeersDiscovered,
                                        color = colors.onSurfaceVariant,
                                    )
                                },
                                onClick = { expanded = false },
                            )
                        } else {
                            nodes.forEach { node ->
                                val custom = customNames[node.id]?.takeIf { it.isNotBlank() }
                                val displayName = custom ?: node.name
                                val subtext = if (custom != null) "${node.name} • ${node.ip}" else node.ip
                                DropdownMenuItem(
                                    onClick = {
                                        expanded = false
                                        onSelectNode(node)
                                    },
                                    text = {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                                            modifier = Modifier.fillMaxWidth(),
                                        ) {
                                            Box(
                                                modifier = Modifier
                                                    .size(8.dp)
                                                    .clip(CircleShape)
                                                    .background(
                                                        if (node.online) palette.onlineGreen
                                                        else palette.stoppedRed,
                                                    ),
                                            )
                                            Column(modifier = Modifier.weight(1f)) {
                                                Text(
                                                    text = displayName,
                                                    color = colors.onSurface,
                                                    fontWeight = FontWeight.Medium,
                                                    maxLines = 1,
                                                    overflow = TextOverflow.Ellipsis,
                                                )
                                                Text(
                                                    text = subtext,
                                                    style = MaterialTheme.typography.labelSmall,
                                                    color = colors.onSurfaceVariant,
                                                    maxLines = 1,
                                                    overflow = TextOverflow.Ellipsis,
                                                )
                                            }
                                            ConnModeBadge(mode = node.connMode)
                                            IconButton(
                                                onClick = {
                                                    expanded = false
                                                    editingNode = node
                                                },
                                                modifier = Modifier.size(28.dp),
                                            ) {
                                                Icon(
                                                    imageVector = Icons.Rounded.Edit,
                                                    contentDescription = strings.renameDevice,
                                                    tint = colors.onSurfaceVariant,
                                                    modifier = Modifier.size(16.dp),
                                                )
                                            }
                                            if (selectedNode?.id == node.id) {
                                                Icon(
                                                    imageVector = Icons.Rounded.Check,
                                                    contentDescription = null,
                                                    tint = colors.primary,
                                                    modifier = Modifier.size(16.dp),
                                                )
                                            }
                                        }
                                    },
                                )
                            }
                        }
                    }
                }
            }
        },
        actions = {
            if (!isSettingsOpen) {
                IconButton(onClick = onRefreshNodes) {
                    Icon(
                        imageVector = Icons.Rounded.Refresh,
                        contentDescription = strings.refreshNodes,
                        tint = colors.onSurfaceVariant,
                    )
                }
                IconButton(onClick = onToggleSettings) {
                    Icon(
                        imageVector = Icons.Rounded.Settings,
                        contentDescription = strings.settingsContentDesc,
                        tint = colors.onSurfaceVariant,
                    )
                }
            }
        },
    )

    editingNode?.let { target ->
        RenameDeviceDialog(
            node = target,
            currentCustomName = customNames[target.id] ?: "",
            onDismiss = { editingNode = null },
            onSave = { newName ->
                onRenameNode(target.id, newName)
                editingNode = null
            },
        )
    }
}

@Composable
fun RenameDeviceDialog(
    node: PeerNode,
    currentCustomName: String,
    onDismiss: () -> Unit,
    onSave: (String) -> Unit,
) {
    val strings = LocalAppStrings.current
    var text by remember(node.id, currentCustomName) { mutableStateOf(currentCustomName) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(strings.renameDevice) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    text = "${node.name} (${node.ip})",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                OutlinedTextField(
                    value = text,
                    onValueChange = { text = it },
                    label = { Text(strings.customNameLabel) },
                    placeholder = { Text(strings.customNamePlaceholder) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        dismissButton = {
            Row {
                if (currentCustomName.isNotBlank()) {
                    TextButton(onClick = { onSave("") }) {
                        Text(strings.resetDefault)
                    }
                }
                TextButton(onClick = onDismiss) {
                    Text(strings.cancel)
                }
            }
        },
        confirmButton = {
            TextButton(onClick = { onSave(text) }) {
                Text(strings.save)
            }
        },
    )
}

@Composable
fun ConnModeBadge(mode: String) {
    val palette = LocalNullWakePalette.current
    val isP2P = mode.equals("P2P", ignoreCase = true)
    val badgeColor = if (isP2P) palette.p2pBadge else palette.derpBadge
    Surface(
        color = badgeColor.copy(alpha = 0.16f),
        shape = RoundedCornerShape(6.dp),
    ) {
        Text(
            text = mode.uppercase(),
            color = badgeColor,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp),
        )
    }
}
