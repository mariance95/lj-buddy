"""Genera assets/icon.ico y assets/icon.png (ejecutar desde la raíz del repo)."""
from PIL import Image, ImageDraw

S = 256
img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
d = ImageDraw.Draw(img)
d.rounded_rectangle((8, 8, S - 8, S - 8), radius=52, fill=(27, 30, 39, 255))
# Cuatro faders a distintas alturas.
for i, level in enumerate((0.35, 0.8, 0.55, 0.95)):
    x = 52 + i * 44
    d.rounded_rectangle((x, 48, x + 20, 208), radius=10, fill=(46, 51, 66, 255))
    top = 208 - int(160 * level)
    d.rounded_rectangle((x, top, x + 20, 208), radius=10, fill=(255, 176, 0, 255))
    d.rounded_rectangle((x - 8, top - 8, x + 28, top + 12), radius=8, fill=(255, 255, 255, 255))
img.save("assets/icon.png")
img.save("assets/icon.ico", sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (256, 256)])
