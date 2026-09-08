// Exercises the production GMMAttach optional-IE method extracted by the runner.
// Minimal checked frame adapter: no native logging globals, subscriber data or RF.
// 测试生产可选 IE 方法，不重写解析循环；适配器只模拟有界字节读取接口。
#include <cassert>
#include <cstddef>
#include <cstdlib>
#include <initializer_list>
#include <iostream>
#include <sstream>
#include <vector>

struct ByteVectorError {};
using ByteVector = std::vector<unsigned char>;
#define SGSNERROR(message) do { std::ostringstream sink; sink << message; } while (false)

struct L3GmmFrame {
    ByteVector bytes;
    explicit L3GmmFrame(ByteVector input) : bytes(input) {}
    size_t size() const { return bytes.size(); }
    const char *hexstr() const { return "synthetic-frame"; } // For the pre-patch negative control.
    unsigned getByte(size_t p) const {
        if (p >= size()) throw ByteVectorError();
        return bytes[p];
    }
    unsigned readByte(size_t &p) { unsigned value = getByte(p); ++p; return value; }
    unsigned readIEI(size_t &p) { return readByte(p); }
    unsigned getField(size_t p, unsigned bits) const {
        unsigned value = 0;
        for (unsigned i = 0; i < bits / 8; ++i) value = (value << 8) | getByte(p + i);
        return value;
    }
    unsigned readUInt16(size_t &p) {
        unsigned value = getField(p, 16); p += 2; return value;
    }
    ByteVector readLVasBV(size_t &p) {
        unsigned length = readByte(p);
        if (length > size() - p) throw ByteVectorError();
        ByteVector result(bytes.begin() + p, bytes.begin() + p + length);
        p += length;
        return result;
    }
    // Native skipLV advances by the declared length; the production loop checks it.
    void skipLV(size_t &p, int, int, const char *) { unsigned length = readByte(p); p += length; }
};
struct MobileIdentity {
    void parseLV(L3GmmFrame &frame, size_t &p) { frame.readLVasBV(p); }
};
struct GMMAttach {
    unsigned mTmsiStatus = 0, mOldPtmsiSignature = 0;
    unsigned mRequestedReadyTimerValue = 0, mDrxParameter = 0;
    MobileIdentity mMobileId, mAdditionalMobileId;
    ByteVector mMsNetworkCapability;
    struct { unsigned mStatus[2] = {0, 0}; } mPdpContextStatus;
    void gmParseIEs(L3GmmFrame &src, size_t &rp, const char *culprit);
};
#include "gmm-optional.inc"

static unsigned checks = 0;
static void check(ByteVector bytes, bool valid, const char *label) {
    L3GmmFrame frame(bytes);
    GMMAttach parsed;
    size_t p = 0;
    bool accepted = false;
    try {
        parsed.gmParseIEs(frame, p, "synthetic-regression");
        accepted = p == frame.size();
    } catch (ByteVectorError) {}
    if (accepted != valid) {
        std::cerr << "FAIL: " << label << '\n';
        std::exit(1);
    }
    ++checks;
}

int main() {
    check({}, true, "no optional IEs");
    for (unsigned base : {0xc0u, 0xd0u, 0xe0u, 0xf0u}) {
        for (unsigned low = 0; low < 16; ++low) {
            unsigned char tv = static_cast<unsigned char>(base | low);
            check({tv}, true, "terminal modern one-octet TV");
            check({tv, 0x58, 2, 0, 0}, true, "modern TV followed by TLV");
        }
    }
    check({0x90, 0xc1, 0xd0, 0xe0, 0xf0, 0x58, 2, 0, 0}, true, "mixed TV and TLV");
    check({0x19, 1, 2, 3, 0x17, 0x44, 0x27, 0, 1}, true, "legacy fixed TV");
    check({0x11, 3, 1, 2, 3, 0x20, 0, 0x40, 3, 1, 2, 3}, true, "legacy TLVs");
    check({0x7f, 2, 1, 2}, true, "unknown well-formed TLV");
    check({0x35, 3, 0, 0, 0}, true, "terminal MBMS has no fallthrough");
    check({0x35, 3, 0, 0, 0, 0x11, 3, 1, 2, 3}, true, "MBMS followed by classmark");
    check({0x32, 2, 1, 2}, true, "PDP context status");
    check({0x58}, false, "missing TLV length");
    check({0x58, 2, 0}, false, "truncated TLV value");
    check({0x7f, 3, 0}, false, "truncated unknown TLV");
    check({0xc1, 0x58, 2, 0}, false, "modern TV must not hide truncated TLV");
    check({0x17}, false, "truncated timer TV");
    check({0x19, 0, 0}, false, "truncated signature TV");
    check({0x27, 0}, false, "truncated DRX TV");
    check({0x32, 1, 0}, false, "truncated PDP status");
    std::cout << "PASS: production GMM optional-IE loop, " << checks << " cases (no RF)\n";
}
