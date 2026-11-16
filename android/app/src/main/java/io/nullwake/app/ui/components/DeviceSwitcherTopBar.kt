package io.nullwake.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.KeyboardArrowDown
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
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
import io.nullwake.app.ui.theme.NullWakeColors

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DeviceSwitcherTopBar(
    nodes: List<PeerNode>,
    selectedNode: PeerNode?,
    isSettingsOpen: Boolean,
    onSelectNode: (PeerNode) -> Unit,
    onRefreshNodes: () -> Unit,
    onToggleSettings: () -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }

    TopAppBar(
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = NullWakeColors.Background,
            titleContentColor = NullWakeColors.TextPrimary,
        ),
        navigationIcon = {
            if (isSettingsOpen) {
                IconButton(onClick = onToggleSettings) {
                    Icon(
                        imageVector = Icons.AutoMirrored.Rounded.ArrowBack,
                        contentDescription = "Back to Dashboard",
                        tint = NullWakeColors.TextPrimary,
                    )
                }
            }
        },
        title = {
            if (isSettingsOpen) {
                Text(
                    text = "Settings",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.SemiBold,
                )
            } else {
                Box {
                    Surface(
                        color = NullWakeColors.SurfaceVariant,
                        shape = RoundedCornerShape(12.dp),
                        modifier = Modifier
                            .clip(RoundedCornerShape(12.dp))
                            .clickable {
                                onRefreshNodes()
                                expanded = true
                            },
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            modifier = Modifier.padding(horizontal = 12.dp, vertical = 7.dp),
                        ) {
                            val dotColor = when {
                                selectedNode == null -> NullWakeColors.TextSecondary
                                selectedNode.online -> NullWakeColors.OnlineGreen
                                else -> NullWakeColors.StoppedRed
                            }
                            Box(
                                modifier = Modifier
                                    .size(9.dp)
                                    .clip(CircleShape)
                                    .background(dotColor),
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            Text(
                                text = selectedNode?.name ?: "Select Host",
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
                                contentDescription = "Switch Node",
                                modifier = Modifier.size(18.dp),
                                tint = NullWakeColors.TextSecondary,
                            )
                        }
                    }

                    DropdownMenu(
                        expanded = expanded,
                        onDismissRequest = { expanded = false },
                        containerColor = NullWakeColors.Surface,
                    ) {
                        if (nodes.isEmpty()) {
                            DropdownMenuItem(
                                text = {
                                    Text(
                                        text = "No tailnet peers discovered yet",
                                        color = NullWakeColors.TextSecondary,
                                    )
                                },
                                onClick = { expanded = false },
                            )
                        } else {
                            nodes.forEach { node ->
                                DropdownMenuItem(
                                    onClick = {
                                        expanded = false
                                        onSelectNode(node)
                                    },
                                    text = {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                                        ) {
                                            Box(
                                                modifier = Modifier
                                                    .size(8.dp)
                                                    .clip(CircleShape)
                                                    .background(
                                                        if (node.online) NullWakeColors.OnlineGreen
                                                        else NullWakeColors.StoppedRed,
                                                    ),
                                            )
                                            Column(modifier = Modifier.weight(1f)) {
                                                Text(
                                                    text = node.name,
                                                    color = NullWakeColors.TextPrimary,
                                                    fontWeight = FontWeight.Medium,
                                                )
                                                Text(
                                                    text = node.ip,
                                                    style = MaterialTheme.typography.labelSmall,
                                                    color = NullWakeColors.TextSecondary,
                                                )
                                            }
                                            ConnModeBadge(mode = node.connMode)
                                            if (selectedNode?.id == node.id) {
                                                Icon(
                                                    imageVector = Icons.Rounded.Check,
                                                    contentDescription = "Selected",
                                                    tint = NullWakeColors.CpuAccent,
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
                        contentDescription = "Refresh Nodes",
                        tint = NullWakeColors.TextSecondary,
                    )
                }
                IconButton(onClick = onToggleSettings) {
                    Icon(
                        imageVector = Icons.Rounded.Settings,
                        contentDescription = "Settings",
                        tint = NullWakeColors.TextSecondary,
                    )
                }
            }
        },
    )
}

@Composable
fun ConnModeBadge(mode: String) {
    val isP2P = mode.equals("P2P", ignoreCase = true)
    val badgeColor = if (isP2P) NullWakeColors.P2PBlue else NullWakeColors.DerpPurple
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
