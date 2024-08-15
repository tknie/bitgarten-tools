ffmpeg -ss 1 -i file.mp4 -vf scale=iw*sar:ih -frames:v 1 1234%03d.jpg

