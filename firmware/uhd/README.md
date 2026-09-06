BlackSDR B210mini clone FPGA image (Artix-7), UHD 4.x era.
Provenance: vendor bundle (B210mini), same file as lte-system
firmware/uhd/blacksdr-B210mini-clone/usrp_b210_fpga.bin.

SHA256: 8E2ACCE1F987D8452D845705C9C27879171724A99B87C640D0E1473E256FE69A
Size: 3825788 bytes (usrp_b210_fpga.blacksdr.bin)

Use: Dockerfile.uhd4 installs it as
/usr/share/uhd/images/usrp_b210_fpga.bin (UHD 4.1 + this board's pair).
Verified on-wire: loads 100% on serial 2548123 (fx3 clean); only the
UHD-3.9 compat number gate rejects it (expected 10, image reports 16).

## B2xx FX3 firmware (UHD 4.x set)

- `usrp_b200_fw.hex` (514346 B) + `usrp_b200_bl.img` (20272 B)
- Provenance: extracted from the working 4.x images set on vm-sdr
  (verified: register loopback passed with these exact files).
- UHD 4.1 requires fw.hex present; without it detection fails with
  "Could not find the image 'usrp_b200_fw.hex'".
