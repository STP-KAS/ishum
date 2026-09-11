"""Cut Ishum explainer: BTCPay beats, facts VO, mixed live UI + filmed stills."""
import subprocess
from pathlib import Path

FF = r"C:\Users\<user>\AppData\Local\Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-9.0.1-full_build\bin\ffmpeg.exe"
HERE = Path(r"C:\Users\<user>\Documents\kaspa\ishum\video")
VID = Path(r"C:\Users\<user>\.grok\sessions\C%3A%5CUsers%5C<user>\01a090a6-b569-7cc3-a35b-b6fb4c556a70\videos")
CLIPS = HERE / "clips"
CLIPS.mkdir(exist_ok=True)

# 58s VO. Ten shots.
SHOTS = [
    ("still", HERE / "card-title.png", 4.0),
    ("move", VID / "1.mp4", 6.0),       # café till
    ("still", HERE / "ui-pay.png", 6.0), # invoice + three rails
    ("move", VID / "3.mp4", 6.0),       # frozen processor
    ("move", VID / "2.mp4", 6.0),       # machine you run
    ("still", HERE / "card-rails.png", 6.0),
    ("move", VID / "5.mp4", 6.0),       # blockDAG
    ("move", VID / "4.mp4", 6.0),       # keys stay with merchant
    ("still", HERE / "ui-pos.png", 6.0),
    ("still", HERE / "card-end.png", 6.0),
]


def run(args):
    subprocess.check_call(args)


def still(src: Path, dst: Path, t: float):
    run([
        FF, "-y", "-loop", "1", "-i", str(src),
        "-t", str(t), "-r", "25",
        "-vf", "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2:#f4efe4,format=yuv420p",
        "-c:v", "libx264", "-pix_fmt", "yuv420p", "-an", str(dst),
    ])


def move(src: Path, dst: Path, t: float):
    run([
        FF, "-y", "-i", str(src),
        "-t", str(t), "-r", "25",
        "-vf", "scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,format=yuv420p",
        "-c:v", "libx264", "-pix_fmt", "yuv420p", "-an", str(dst),
    ])


def main():
    files = []
    for i, (kind, src, t) in enumerate(SHOTS):
        dst = CLIPS / f"{i:02d}.mp4"
        if kind == "still":
            still(src, dst, t)
        else:
            move(src, dst, t)
        files.append(dst)
    lst = CLIPS / "list.txt"
    lst.write_text("".join(f"file '{p.as_posix()}'\n" for p in files), encoding="utf-8")
    picture = HERE / "picture.mp4"
    run([
        FF, "-y", "-f", "concat", "-safe", "0", "-i", str(lst),
        "-c:v", "libx264", "-pix_fmt", "yuv420p", "-r", "25", str(picture),
    ])
    out = HERE / "Ishum-sequenced-on-Kaspa.mp4"
    run([
        FF, "-y", "-i", str(picture), "-i", str(HERE / "vo.wav"),
        "-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
        "-shortest", str(out),
    ])
    print(out)


if __name__ == "__main__":
    main()
