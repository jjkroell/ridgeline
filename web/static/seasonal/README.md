Seasonal live-map icons, from Noto Emoji (https://github.com/googlefonts/noto-emoji,
2D/svg), Apache License 2.0. Bundled so the icons look the same on every device,
including ones without a colour emoji font. Named emoji_u<codepoints>.svg with
U+FE0F dropped, as `seasonal.src()` expects; width/height were added to the <svg>
tag so Firefox can draw them onto a canvas.

To add icons for a new season: fetch 2D/svg/emoji_u<codepoints>.svg from the
repo above (codepoints joined by "_", without fe0f), then add
width="128" height="128" to its <svg> tag.
