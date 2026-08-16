package com.tokenflow.chat.ui.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalView
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat

private val LightColors = lightColorScheme(
    primary = Color(0xFF0B7468),
    onPrimary = Color.White,
    primaryContainer = Color(0xFFE2F1EC),
    onPrimaryContainer = Color(0xFF123A34),
    secondary = Color(0xFF496B63),
    onSecondary = Color.White,
    secondaryContainer = Color(0xFFE2F1EC),
    onSecondaryContainer = Color(0xFF173D36),
    tertiary = Color(0xFF176B87),
    onTertiary = Color.White,
    tertiaryContainer = Color(0xFFE5F0F4),
    onTertiaryContainer = Color(0xFF153941),
    background = Color(0xFFFAFCFA),
    surface = Color(0xFFFAFCFA),
    surfaceBright = Color(0xFFFAFCFA),
    surfaceDim = Color(0xFFE9EFEC),
    surfaceContainerLowest = Color(0xFFFFFFFF),
    surfaceContainerLow = Color(0xFFF5F8F6),
    surfaceContainer = Color(0xFFF0F5F2),
    surfaceContainerHigh = Color(0xFFEAF1ED),
    surfaceContainerHighest = Color(0xFFE4ECE8),
    surfaceVariant = Color(0xFFEEF4F1),
    outline = Color(0xFFCBD8D3),
    outlineVariant = Color(0xFFE0E8E4),
    error = Color(0xFFBA1A1A),
)

private val DarkColors = darkColorScheme(
    primary = Color(0xFF7ED3C1),
    onPrimary = Color(0xFF00382F),
    primaryContainer = Color(0xFF214B43),
    onPrimaryContainer = Color(0xFFB7F1E3),
    secondary = Color(0xFF9ED1BD),
    onSecondary = Color(0xFF06372D),
    secondaryContainer = Color(0xFF29443B),
    onSecondaryContainer = Color(0xFFC2E9DD),
    tertiary = Color(0xFF88CEEA),
    onTertiary = Color(0xFF003544),
    tertiaryContainer = Color(0xFF243F49),
    onTertiaryContainer = Color(0xFFC4EAF7),
    background = Color(0xFF101613),
    surface = Color(0xFF151D19),
    surfaceBright = Color(0xFF29342F),
    surfaceDim = Color(0xFF101613),
    surfaceContainerLowest = Color(0xFF0C110F),
    surfaceContainerLow = Color(0xFF131A17),
    surfaceContainer = Color(0xFF17201C),
    surfaceContainerHigh = Color(0xFF1C2722),
    surfaceContainerHighest = Color(0xFF24312B),
    surfaceVariant = Color(0xFF202B26),
    outline = Color(0xFF53655E),
    outlineVariant = Color(0xFF2B3732),
    error = Color(0xFFFFB4AB),
)

private val JadeShapes = Shapes(
    extraSmall = RoundedCornerShape(4.dp),
    small = RoundedCornerShape(6.dp),
    medium = RoundedCornerShape(8.dp),
    large = RoundedCornerShape(8.dp),
    extraLarge = RoundedCornerShape(8.dp),
)

@Composable
fun TokenFlowTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    val colors = if (dark) DarkColors else LightColors
    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as Activity).window
            window.statusBarColor = Color.Transparent.toArgb()
            window.navigationBarColor = colors.background.toArgb()
            WindowCompat.getInsetsController(window, view).apply {
                isAppearanceLightStatusBars = !dark
                isAppearanceLightNavigationBars = !dark
            }
        }
    }
    MaterialTheme(colorScheme = colors, shapes = JadeShapes, content = content)
}
