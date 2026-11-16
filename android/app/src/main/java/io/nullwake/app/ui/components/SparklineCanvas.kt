package io.nullwake.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import io.nullwake.app.ui.theme.NullWakeColors

/**
 * Lightweight 60-point rolling sparkline drawn directly on Compose Canvas with cubic Bezier smoothing.
 */
@Composable
fun SparklineCanvas(
    points: List<Float>,
    lineColor: Color,
    modifier: Modifier = Modifier,
    height: Dp = 72.dp,
    maxValue: Float? = 100f,
    secondaryPoints: List<Float>? = null,
    secondaryColor: Color = NullWakeColors.NetTxAccent,
) {
    Canvas(
        modifier = modifier
            .fillMaxWidth()
            .height(height),
    ) {
        val width = size.width
        val canvasHeight = size.height
        if (width <= 0f || canvasHeight <= 0f) return@Canvas

        val padTop = 6.dp.toPx()
        val padBottom = 6.dp.toPx()
        val usableHeight = (canvasHeight - padTop - padBottom).coerceAtLeast(1f)

        // Subtle horizontal reference lines at 100% (top), 50% (mid), and baseline.
        val dashEffect = PathEffect.dashPathEffect(floatArrayOf(5f, 7f), 0f)
        drawLine(
            color = NullWakeColors.Border.copy(alpha = 0.55f),
            start = Offset(0f, padTop),
            end = Offset(width, padTop),
            strokeWidth = 1.dp.toPx(),
            pathEffect = dashEffect,
        )
        drawLine(
            color = NullWakeColors.Border.copy(alpha = 0.55f),
            start = Offset(0f, padTop + usableHeight * 0.5f),
            end = Offset(width, padTop + usableHeight * 0.5f),
            strokeWidth = 1.dp.toPx(),
            pathEffect = dashEffect,
        )
        drawLine(
            color = NullWakeColors.Border.copy(alpha = 0.4f),
            start = Offset(0f, padTop + usableHeight),
            end = Offset(width, padTop + usableHeight),
            strokeWidth = 1.dp.toPx(),
        )

        val combinedMax = maxValue ?: run {
            val m1 = points.maxOrNull() ?: 1f
            val m2 = secondaryPoints?.maxOrNull() ?: 0f
            maxOf(m1, m2, 1f) * 1.15f
        }

        fun drawSeries(series: List<Float>, color: Color, fillGradient: Boolean) {
            if (series.isEmpty()) return
            val maxSlots = 59f // 60-point rolling window anchored to right edge
            val startIndex = (60 - series.size).coerceAtLeast(0)

            val coords = series.mapIndexed { idx, value ->
                val x = ((startIndex + idx) / maxSlots) * width
                val norm = (value / combinedMax).coerceIn(0f, 1f)
                val y = padTop + usableHeight * (1f - norm)
                Offset(x, y)
            }

            if (coords.size == 1) {
                drawCircle(color = color, radius = 3.dp.toPx(), center = coords.first())
                return
            }

            val linePath = Path().apply {
                moveTo(coords.first().x, coords.first().y)
                for (i in 0 until coords.lastIndex) {
                    val p0 = coords[i]
                    val p1 = coords[i + 1]
                    val midX = (p0.x + p1.x) / 2f
                    cubicTo(midX, p0.y, midX, p1.y, p1.x, p1.y)
                }
            }

            if (fillGradient) {
                val fillPath = Path().apply {
                    addPath(linePath)
                    lineTo(coords.last().x, padTop + usableHeight)
                    lineTo(coords.first().x, padTop + usableHeight)
                    close()
                }
                drawPath(
                    path = fillPath,
                    brush = Brush.verticalGradient(
                        0.0f to color.copy(alpha = 0.34f),
                        0.6f to color.copy(alpha = 0.10f),
                        1.0f to color.copy(alpha = 0.0f),
                        startY = padTop,
                        endY = padTop + usableHeight,
                    ),
                )
            }

            drawPath(
                path = linePath,
                color = color,
                style = Stroke(
                    width = 2.2.dp.toPx(),
                    cap = StrokeCap.Round,
                    join = StrokeJoin.Round,
                ),
            )

            // Highlight latest sample point.
            val last = coords.last()
            drawCircle(color = color.copy(alpha = 0.25f), radius = 5.dp.toPx(), center = last)
            drawCircle(color = color, radius = 2.6.dp.toPx(), center = last)
        }

        if (!secondaryPoints.isNullOrEmpty()) {
            drawSeries(secondaryPoints, secondaryColor, fillGradient = false)
        }
        drawSeries(points, lineColor, fillGradient = true)
    }
}
