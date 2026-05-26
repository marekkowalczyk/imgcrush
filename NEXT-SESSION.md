# Next Session

- **PNG palette optimization** — highest-value backlog item. Benchmark showed ImageOptim gets 80% on few-color PNGs where imgcrush gets 23%. Investigate `esimov/colorquant` or `delthas/octreequant` for color quantization. See RESEARCH.md section 3 and SPEC.md Priority 1.
- **Lossless JPEG optimization** — research whether a pure-Go Huffman table optimizer or progressive JPEG encoder is feasible. Currently imgcrush has nothing for lossless JPEG. See SPEC.md Priority 2.
- **Real-world testing** — run imgcrush on a larger batch of real photos and screenshots to build more baseline data. Use `testdata/benchmark.sh` to compare against ImageOptim.
