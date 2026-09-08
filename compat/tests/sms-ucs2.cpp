// Production TLUserData methods with a bounds-checking BitVector test double.
// 提取生产解码/读写方法，使用有边界检查的位向量；不运行网络或射频。
#include <algorithm>
#include <cassert>
#include <iostream>
#include <memory>
#include <sstream>
#include <stdexcept>
#include <string>
#include <vector>

struct SMSReadError {};
#define SMS_READ_ERROR {throw SMSReadError();}
#define LOG(level) std::ostringstream()

class BitVector {
    std::shared_ptr<std::vector<unsigned>> bits;
    size_t offset = 0, count = 0;
public:
    explicit BitVector(size_t n=0): bits(new std::vector<unsigned>(n)), count(n) {}
    size_t size() const { return count; }
    BitVector alias() const { return *this; }
    BitVector tail(size_t n) const {
        if (n > count) throw std::out_of_range("tail");
        BitVector result = *this;
        result.offset += n; result.count -= n;
        return result;
    }
    unsigned readField(size_t& p, unsigned n) const {
        if (p+n > count) throw std::out_of_range("read");
        unsigned result=0;
        while (n--) result = (result<<1) | bits->at(offset+p++);
        return result;
    }
    unsigned readFieldReversed(size_t& p, unsigned n) const {
        if (p+n > count) throw std::out_of_range("read reversed");
        unsigned result=0;
        for (unsigned i=0; i<n; ++i) result |= bits->at(offset+p++) << i;
        return result;
    }
    unsigned peekFieldReversed(size_t p, unsigned n) const { return readFieldReversed(p,n); }
    void writeField(size_t& p, unsigned value, unsigned n) {
        if (p+n > count) throw std::out_of_range("write");
        while (n--) bits->at(offset+p++) = (value>>n)&1;
    }
    void LSB8MSB() {
        for (size_t p=0; p+8<=count; p+=8)
            std::reverse(bits->begin()+offset+p,bits->begin()+offset+p+8);
    }
    void copyTo(BitVector& dest) const {
        if (count > dest.count) throw std::out_of_range("copy");
        for (size_t i=0; i<count; ++i) dest.bits->at(dest.offset+i)=bits->at(offset+i);
    }
    void clone(const BitVector& other) { *this=BitVector(other.size()); other.copyTo(*this); }
};
using TLFrame = BitVector;
// This harness only exercises ordinary ASCII for the unchanged GSM7 branch.
static char decodeGSMChar(unsigned gsm) { return static_cast<char>(gsm); }
class TLUserData {
    unsigned mDCS;
    bool mUDHI;
    unsigned mLength=0;
    BitVector mRawData;
public:
    TLUserData(unsigned dcs, bool udhi=false): mDCS(dcs), mUDHI(udhi) {}
    unsigned DCS() const { return mDCS; }
    bool UDHI() const { return mUDHI; }
    void parse(const TLFrame&,size_t&);
    void write(TLFrame&,size_t&) const;
    std::string decode() const;
};
#include "sms-ucs2-methods.inc"

static TLFrame wire(unsigned length, const std::vector<unsigned>& bytes) {
    TLFrame frame((bytes.size()+1)*8);
    size_t p=0;
    frame.writeField(p,length,8);
    for (unsigned byte: bytes) frame.writeField(p,byte,8);
    return frame;
}
static TLUserData parsed(unsigned dcs, bool udhi, unsigned length, const std::vector<unsigned>& bytes) {
    TLUserData ud(dcs,udhi);
    TLFrame frame=wire(length,bytes);
    size_t p=0;
    ud.parse(frame,p);
    return ud;
}
static void expectRejected(unsigned dcs, bool udhi, unsigned length, const std::vector<unsigned>& bytes) {
    try { parsed(dcs,udhi,length,bytes).decode(); }
    catch (const SMSReadError&) { return; }
    throw std::runtime_error("malformed/unsupported payload was decoded");
}
static void expectText(unsigned dcs, bool udhi, unsigned length,
                       const std::vector<unsigned>& bytes, const std::string& expected) {
    TLUserData ud=parsed(dcs,udhi,length,bytes);
    if (ud.decode()!=expected) throw std::runtime_error("decoded text mismatch");
    // TLDeliver copies the original UD, not decoded UTF-8. Ensure decoding
    // leaves DCS, UDHI, UDL and the complete packed payload unchanged.
    TLUserData forwarded=ud;
    if (forwarded.DCS()!=dcs || forwarded.UDHI()!=udhi) throw std::runtime_error("metadata changed");
    TLFrame frame((bytes.size()+1)*8);
    size_t p=0;
    forwarded.write(frame,p);
    p=0;
    if (frame.readField(p,8)!=length) throw std::runtime_error("UDL changed");
    for (unsigned byte: bytes)
        if (frame.readField(p,8)!=byte) throw std::runtime_error("forwarded raw payload changed");
}
int main() {
    try {
        expectText(0,false,1,{0x41},"A");
        const std::string chinese="\xe4\xb8\xad\xe6\x96\x87";
        try { expectText(8,false,4,{0x4e,0x2d,0x65,0x87},chinese); }
        catch (const SMSReadError&) {
            std::cerr << "EXPECTED-NEGATIVE: original rejects DCS 0x08\n";
            return 2;
        }
        for (unsigned dcs: {8u,24u,25u,26u,27u})
            expectText(dcs,false,4,{0x4e,0x2d,0x65,0x87},chinese);
        expectText(8,false,8,{0,0x41,0,0xe9,0x03,0xa9,0x20,0xac},"A\xc3\xa9\xce\xa9\xe2\x82\xac");
        expectText(8,false,0,{},"");
        expectText(8,false,2,{0,0},std::string(1,'\0'));
        expectText(8,true,10,{5,0,3,0x11,2,1,0x4e,0x2d,0x65,0x87},chinese);
        expectText(8,true,11,{6,8,4,0x12,0x34,2,1,0x4e,0x2d,0x65,0x87},chinese);
        expectText(8,true,6,{5,0,3,0x11,2,1},"");
        expectText(8,false,2,{0x4e,0x2d,0,0},"\xe4\xb8\xad"); // TP-UDL is authoritative.
        std::vector<unsigned> maxBytes;
        std::string maxText;
        for (unsigned i=0; i<70; ++i) { maxBytes.push_back(0x4e); maxBytes.push_back(0x2d); maxText+="\xe4\xb8\xad"; }
        expectText(8,false,140,maxBytes,maxText);
        expectRejected(8,false,4,{0x4e,0x2d});
        expectRejected(8,false,1,{0x4e});
        expectRejected(8,false,142,std::vector<unsigned>(142));
        expectRejected(8,true,0,{});
        expectRejected(8,true,2,{4,0});
        expectRejected(8,true,3,{1,0,0});
        expectRejected(8,false,2,{0xd8,0});
        expectRejected(8,false,2,{0xdc,0});
        expectRejected(8,false,4,{0xd8,0x3d,0xde,0});
        for (unsigned dcs: {4u,12u,40u,72u,224u,255u})
            expectRejected(dcs,false,4,{0x4e,0x2d,0x65,0x87});
        std::cout << "PASS: UCS-2 decode, malformed/unsupported rejection, GSM7 and wire preservation\n";
    } catch (const std::exception& error) {
        std::cerr << "FAIL: " << error.what() << '\n'; return 1;
    } catch (const SMSReadError&) {
        std::cerr << "FAIL: valid UCS-2 rejected\n"; return 1;
    }
}
