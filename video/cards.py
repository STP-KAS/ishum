"""Title cards with exact text. Paper/ink of the Ishum till."""
from PIL import Image, ImageDraw, ImageFont

OUT = r"C:\Users\Remco\Documents\kaspa\ishum\video"
W, H = 1920, 1080
PAPER = (244, 239, 228)
INK = (20, 28, 24)
MUTED = (92, 107, 100)
ACCENT = (14, 122, 102)


def font(size):
    for name in ("segoeui.ttf", "SegoeUI.ttf", "arial.ttf", "calibri.ttf"):
        try:
            return ImageFont.truetype(name, size)
        except OSError:
            continue
    return ImageFont.load_default()


def card(path, title, sub, extra=None):
    im = Image.new("RGB", (W, H), PAPER)
    d = ImageDraw.Draw(im)
    ft = font(96)
    fs = font(36)
    fx = font(28)
    tw = d.textlength(title, font=ft)
    d.text(((W - tw) / 2, 400), title, font=ft, fill=INK)
    sw = d.textlength(sub, font=fs)
    d.text(((W - sw) / 2, 520), sub, font=fs, fill=ACCENT)
    if extra:
        ew = d.textlength(extra, font=fx)
        d.text(((W - ew) / 2, 620), extra, font=fx, fill=MUTED)
    im.save(path)


if __name__ == "__main__":
    card(f"{OUT}/card-title.png", "Ishum", "sequenced on Kaspa")
    card(
        f"{OUT}/card-rails.png",
        "Three rails",
        "Kaspa native  ·  Kaspa stable  ·  Kaspa stable alternative",
        "Only KAS is live. The two dollar rails are reserved slots.",
    )
    card(f"{OUT}/card-end.png", "Ishum", "sequenced on Kaspa", "Self-hosted. No keys on the server.")
    print("ok")
