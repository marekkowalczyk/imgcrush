# Next Session

- **Implement metadata preservation (phase 1)** — raw byte-level splicing of JPEG APP segments (APP1/EXIF, APP1/XMP, APP2/ICC, APP13/IPTC) and PNG metadata chunks (eXIf, iCCP, iTXt, tEXt, zTXt). Add `--keep-metadata` flag. Copy segments as opaque blobs, no EXIF parsing needed. See RESEARCH.md section 5.
- **PNG palette optimization** — still the biggest compression gap from benchmarks. Investigate `esimov/colorquant` or `delthas/octreequant` for color quantization. See RESEARCH.md section 3 and SPEC.md Priority 1.
- **Learn Alan Barnard's Pro-Con-Cloud method** — user wanted this analysis applied but we didn't know the actual method. Research it for future use.
