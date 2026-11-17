package io.nullwake.app.ui.theme

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.shape.RoundedCornerShape

enum class ThemeMode(val key: String) {
    SYSTEM("system"),
    LIGHT("light"),
    DARK("dark");

    companion object {
        fun fromKey(key: String?): ThemeMode =
            entries.firstOrNull { it.key == key } ?: SYSTEM
    }
}

@Immutable
data class NullWakePalette(
    val cpuAccent: Color,
    val ramAccent: Color,
    val gpuAccent: Color,
    val netRxAccent: Color, // Downlink (↓)
    val netTxAccent: Color, // Uplink (↑)
    val onlineGreen: Color,
    val degradedAmber: Color,
    val stoppedRed: Color,
    val p2pBadge: Color,
    val derpBadge: Color,
    val cardBorder: Color,
)

private val DarkPalette = NullWakePalette(
    cpuAccent = Color(0xFF22D3EE),     // Cyan
    ramAccent = Color(0xFF34D399),     // Emerald
    gpuAccent = Color(0xFFFBBF24),     // Amber
    netRxAccent = Color(0xFFA78BFA),   // Violet (Download)
    netTxAccent = Color(0xFFF472B6),   // Rose Pink (Upload)
    onlineGreen = Color(0xFF22C55E),
    degradedAmber = Color(0xFFF59E0B),
    stoppedRed = Color(0xFFEF4444),
    p2pBadge = Color(0xFF22D3EE),
    derpBadge = Color(0xFFA78BFA),
    cardBorder = Color(0xFF2A3644),
)

private val AmoledPalette = DarkPalette.copy(
    cardBorder = Color(0xFF1F2732),
)

private val LightPalette = NullWakePalette(
    cpuAccent = Color(0xFF0891B2),     // Deep Cyan
    ramAccent = Color(0xFF059669),     // Deep Emerald
    gpuAccent = Color(0xFFD97706),     // Deep Amber
    netRxAccent = Color(0xFF7C3AED),   // Deep Violet (Download)
    netTxAccent = Color(0xFFDB2777),   // Deep Rose (Upload)
    onlineGreen = Color(0xFF16A34A),
    degradedAmber = Color(0xFFD97706),
    stoppedRed = Color(0xFFDC2626),
    p2pBadge = Color(0xFF0284C7),
    derpBadge = Color(0xFF7C3AED),
    cardBorder = Color(0xFFD8E0EA),
)

val LocalNullWakePalette = staticCompositionLocalOf { DarkPalette }

private val NullWakeDarkScheme = darkColorScheme(
    primary = Color(0xFF22D3EE),
    onPrimary = Color(0xFF062028),
    primaryContainer = Color(0xFF123846),
    onPrimaryContainer = Color(0xFFCFFAFE),
    secondary = Color(0xFF34D399),
    onSecondary = Color(0xFF05291C),
    tertiary = Color(0xFFA78BFA),
    background = Color(0xFF0F1419),
    onBackground = Color(0xFFF1F5F9),
    surface = Color(0xFF182028),
    onSurface = Color(0xFFF1F5F9),
    surfaceVariant = Color(0xFF212B36),
    onSurfaceVariant = Color(0xFF94A3B8),
    surfaceContainerLow = Color(0xFF141A21),
    surfaceContainer = Color(0xFF182028),
    surfaceContainerHigh = Color(0xFF1F2933),
    outline = Color(0xFF2A3644),
    outlineVariant = Color(0xFF24303D),
)

private val NullWakeLightScheme = lightColorScheme(
    primary = Color(0xFF0891B2),
    onPrimary = Color.White,
    primaryContainer = Color(0xFFCFFAFE),
    onPrimaryContainer = Color(0xFF083344),
    secondary = Color(0xFF059669),
    onSecondary = Color.White,
    tertiary = Color(0xFF7C3AED),
    background = Color(0xFFF4F7FB),
    onBackground = Color(0xFF0F172A),
    surface = Color(0xFFFFFFFF),
    onSurface = Color(0xFF0F172A),
    surfaceVariant = Color(0xFFE7EEF6),
    onSurfaceVariant = Color(0xFF475569),
    surfaceContainerLow = Color(0xFFF8FAFC),
    surfaceContainer = Color(0xFFFFFFFF),
    surfaceContainerHigh = Color(0xFFEFF4FA),
    outline = Color(0xFFCBD5E1),
    outlineVariant = Color(0xFFE2E8F0),
)

private val NullWakeShapes = Shapes(
    extraSmall = RoundedCornerShape(6.dp),
    small = RoundedCornerShape(10.dp),
    medium = RoundedCornerShape(16.dp),
    large = RoundedCornerShape(20.dp),
    extraLarge = RoundedCornerShape(28.dp),
)

private fun ColorScheme.withAmoledBlack(): ColorScheme = copy(
    background = Color.Black,
    surface = Color(0xFF0A0D12),
    surfaceContainerLowest = Color.Black,
    surfaceContainerLow = Color(0xFF07090D),
    surfaceContainer = Color(0xFF0C1016),
    surfaceContainerHigh = Color(0xFF121720),
    surfaceContainerHighest = Color(0xFF181F2B),
    surfaceVariant = Color(0xFF141A23),
    outline = Color(0xFF1F2732),
    outlineVariant = Color(0xFF1A212B),
)

@Composable
fun NullWakeTheme(
    themeMode: ThemeMode = ThemeMode.SYSTEM,
    amoledBlack: Boolean = false,
    dynamicColor: Boolean = true,
    content: @Composable () -> Unit,
) {
    val isDark = when (themeMode) {
        ThemeMode.SYSTEM -> isSystemInDarkTheme()
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
    }

    val context = LocalContext.current
    val baseScheme = when {
        dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
            if (isDark) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
        }
        isDark -> NullWakeDarkScheme
        else -> NullWakeLightScheme
    }

    val finalScheme = if (isDark && amoledBlack) baseScheme.withAmoledBlack() else baseScheme
    val palette = when {
        !isDark -> LightPalette
        amoledBlack -> AmoledPalette
        else -> DarkPalette
    }

    CompositionLocalProvider(LocalNullWakePalette provides palette) {
        MaterialTheme(
            colorScheme = finalScheme,
            shapes = NullWakeShapes,
            content = content,
        )
    }
}
