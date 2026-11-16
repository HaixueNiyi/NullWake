package io.nullwake.app.ui.theme

import android.os.Build
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext

object NullWakeColors {
    val Background = Color(0xFF0F1419)     // Deep slate
    val Surface = Color(0xFF182028)        // Elevated card surface
    val SurfaceVariant = Color(0xFF212B36) // Subtle elevated surface
    val Border = Color(0xFF2A3644)         // Subtle 1dp outline border
    val TextPrimary = Color(0xFFF1F5F9)
    val TextSecondary = Color(0xFF94A3B8)

    // Distinct semantic telemetry chart colors
    val CpuAccent = Color(0xFF22D3EE)      // Cyan
    val RamAccent = Color(0xFF34D399)      // Emerald
    val GpuAccent = Color(0xFFFBBF24)      // Amber
    val NetAccent = Color(0xFFA78BFA)      // Violet
    val NetRxAccent = NetAccent
    val NetTxAccent = Color(0xFFC084FC)    // Light Violet

    val GpuAmdAccent = GpuAccent
    val GpuIntelAccent = GpuAccent

    val OnlineGreen = Color(0xFF22C55E)
    val DegradedAmber = Color(0xFFF59E0B)
    val StoppedRed = Color(0xFFEF4444)
    val P2PBlue = Color(0xFF22D3EE)
    val DerpPurple = Color(0xFFA78BFA)
}

private val NullWakeDarkScheme = darkColorScheme(
    primary = NullWakeColors.CpuAccent,
    onPrimary = Color(0xFF062028),
    secondary = NullWakeColors.RamAccent,
    tertiary = NullWakeColors.NetAccent,
    background = NullWakeColors.Background,
    onBackground = NullWakeColors.TextPrimary,
    surface = NullWakeColors.Surface,
    onSurface = NullWakeColors.TextPrimary,
    surfaceVariant = NullWakeColors.SurfaceVariant,
    onSurfaceVariant = NullWakeColors.TextSecondary,
    outline = NullWakeColors.Border,
)

private val NullWakeLightScheme = lightColorScheme(
    primary = Color(0xFF0891B2),
    secondary = Color(0xFF059669),
    tertiary = Color(0xFF7C3AED),
)

@Composable
fun NullWakeTheme(
    darkTheme: Boolean = true,
    dynamicColor: Boolean = true,
    content: @Composable () -> Unit,
) {
    val context = LocalContext.current
    val colorScheme = when {
        dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
            if (darkTheme) {
                dynamicDarkColorScheme(context).copy(
                    background = NullWakeColors.Background,
                    onBackground = NullWakeColors.TextPrimary,
                    surface = NullWakeColors.Surface,
                    onSurface = NullWakeColors.TextPrimary,
                    surfaceVariant = NullWakeColors.SurfaceVariant,
                    onSurfaceVariant = NullWakeColors.TextSecondary,
                    outline = NullWakeColors.Border,
                )
            } else {
                dynamicLightColorScheme(context)
            }
        }
        darkTheme -> NullWakeDarkScheme
        else -> NullWakeLightScheme
    }

    MaterialTheme(
        colorScheme = colorScheme,
        content = content,
    )
}
