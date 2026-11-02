Seasonal live-map icons, from Noto Emoji (https://github.com/googlefonts/noto-emoji,
2D/svg), Apache License 2.0. Bundled so the icons look the same on every device,
including ones without a colour emoji font. Named emoji_u<codepoints>.svg with
U+FE0F dropped, as `seasonal.src()` expects; width/height were added to the <svg>
tag so Firefox can draw them onto a canvas.

To add icons for a new season: fetch 2D/svg/emoji_u<codepoints>.svg from the
repo above (codepoints joined by "_", without fe0f), then add
width="128" height="128" to its <svg> tag.

sleigh.svg (Christmas adverts) is our own composite of three Noto Emoji
pictures — reindeer U+1F98C, sled U+1F6F7 and Santa U+1F385 — inlined as nested
<svg>s (ids prefixed so they can't clash) plus two drawn reins; same licence.
It faces LEFT; `CUSTOM` in lib/seasonal.svelte.ts sets its map size and mirrors
it while the packet heads east.

ho.mp3 (Christmas: played as Santa's sleigh lands) is one "ho" cut from a
"ho ho ho" clip Jesse supplied (2026-11-01): 0.97–1.80 s of hohoho.mp3, mono,
15 ms fade-in, 130 ms fade-out, peak normalised to −3 dB, 96 kb/s.
