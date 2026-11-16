package io.nullwake.app.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

object NullWakeColors {
    val Background = Color(0xFF0B0F17)
    val Surface = Color(0xFF131A26)
    val SurfaceVariant = Color(0xFF1B2434)
    val Border = Color(0xFF273247)
    val TextPrimary = Color(0xFFF1F5F9)
    val TextSecondary = Color(0xFF94A3B8)

    val CpuAccent = Color(0xFF38BDF8)      // Sky cyan
    val RamAccent = Color(0xFFA78BFA)      // Violet
    val NetRxAccent = Color(0xFF34D399)    // Emerald
    val NetTxAccent = Color(0xFFFBBF24)    // Amber
    val GpuAmdAccent = Color(0xFFF87171)   // Coral red
    val GpuIntelAccent = Color(0xFF60A5FA) // Intel blue

    val OnlineGreen = Color(0xFF22C55E)
    val DegradedAmber = Color(0xFFF59E0B)
    val StoppedRed = Color(0xFFEF4444)
    val P2PBlue = Color(0xFF38BDF8)
    val DerpPurple = Color(0xFFC084FC)
}

private val NullWakeDarkScheme = darkColorScheme(
    primary = NullWakeColors.CpuAccent,
    onPrimary = Color(0xFF062030),
    secondary = NullWakeColors.RamAccent,
    tertiary = NullWakeColors.NetRxAccent,
    background = NullWakeColors.Background,
    onBackground = NullWakeColors.TextPrimary,
    surface = NullWakeColors.Surface,
    onSurface = NullWakeColors.TextPrimary,
    surfaceVariant = NullWakeColors.SurfaceVariant,
    onSurfaceVariant = NullWakeColors.TextSecondary,
    outline = NullWakeColors.Border,
)

@Composable
fun NullWakeTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = NullWakeDarkScheme,
        content = content,
    )
}
