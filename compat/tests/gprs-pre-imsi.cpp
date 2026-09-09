// Exercise extracted production constructors, TLLI assignment and public pager.
// 提取真实构造函数、TLLI 指派和公开版寻呼分支；不启动 RF/服务。
#include <cassert>
#include <cstdint>
#include <cstdlib>
#include <deque>
#include <iostream>
#include <string>
using std::string;
struct NullLog { template<class T> NullLog& operator<<(const T&) { return *this; } };
#define LOG(level) NullLog()
#define GPRSLOG(level) NullLog()
#define LOGVAR(value) value
#define LOGHEX(value) value
#define LOGWATCHF(...) do {} while (0)
struct Int_z { int n; Int_z():n(0) {} int operator++() { return ++n; } };
struct Bool_z { bool n; Bool_z():n(false) {} };
struct TMSI_t {};
static int aliveAssignments=0, gsmIdentityLookups=0, gsmSends=0;
namespace GPRS { struct TBF; }
namespace GSM {
enum ChannelType { PSingleBlock1PhaseType, SDCCHType };
struct Time { unsigned n; Time(unsigned v=0,int=0):n(v) {} unsigned FN() const { return n; } Time operator+(unsigned v) const { return Time(n+v); } };
struct L3RequestReference { L3RequestReference(int,Time) {} };
struct L3TimingAdvance { explicit L3TimingAdvance(int) {} };
struct L3IAPacketAssignment {
    uint32_t tlli=0; unsigned tfi=0;
    void setPacketPowerOptions(int,int) {}
    void setPacketDownlinkAssign(uint32_t l,unsigned f,int,bool,int) { tlli=l; tfi=f; }
};
struct L3ImmediateAssignment {
    L3IAPacketAssignment packet;
    L3ImmediateAssignment(L3RequestReference,int,L3TimingAdvance,bool tbf,bool down) { assert(tbf && down); ++aliveAssignments; }
    ~L3ImmediateAssignment() { --aliveAssignments; }
    L3IAPacketAssignment* packetAssign() { return &packet; }
};
struct L3MobileIdentity {};
struct L3PagingRequestType1 { L3PagingRequestType1(const L3MobileIdentity&,ChannelType type) { assert(type==SDCCHType); } };
}
namespace Control {
#include "paging-entry.inc"
#include "paging-destructor.inc"
GSM::ChannelType NewPagingEntry::getGsmChanType() const { return mInitialChanType; }
GSM::L3MobileIdentity NewPagingEntry::getMobileId() { ++gsmIdentityLookups; assert(!mImsi.empty()); return GSM::L3MobileIdentity(); }
}
using namespace Control;
using namespace GSM;
struct RLCBSN_t { unsigned n; RLCBSN_t(unsigned v):n(v) {} RLCBSN_t operator+(unsigned v) const { return RLCBSN_t(n+v); } };
static RLCBSN_t gBSNNext(100);
static unsigned BSN2FrameNumber(RLCBSN_t b) { return b.n; }
struct BTS { GSM::Time time() { return GSM::Time(1234); } } gBTS;
static int GetPowerAlpha() { return 1; }
static int GetPowerGamma() { return 2; }
namespace GPRS {
namespace RLCDir { enum Dir { Up,Down }; }
enum MsgState { MsgTransAssign1 };
static bool gFixConvertForeignTLLI=false;
static const uint32_t TLLI_LOCAL_BIT=0x40000000;
struct MSInfo {
    string imsi; uint32_t handle=0x80000001;
    uint32_t msGetHandle() { return handle; }
    string sgsnFindImsiByHandle(uint32_t h) { assert(h==handle); return imsi; }
    int msGetTA() { return 1; }
};
struct PDCHL1FEC { int packetChannelDescription() { return 0; } };
struct TBF {
    MSInfo* mtMS; RLCDir::Dir mtDir=RLCDir::Down;
    uint32_t mtTlli=0x80000001; unsigned mtTFI=7;
    bool mtUnAckMode=false; unsigned mtAssignCounter=0,mtCcchAssignCounter=0,acks=0;
    uint32_t mtGetTlli();
    int mtChannelCoding() { return 1; }
    void mtSetAckExpected(RLCBSN_t b,MsgState) { assert(b.n==1100); ++acks; }
};
static void tbfDumpAll() {}
}
template<class T> struct Queue {
    std::deque<T*> entries;
    T* readNoBlock() { if(entries.empty()) return NULL; T* p=entries.front(); entries.pop_front(); return p; }
    void write_front(T* p) { entries.push_front(p); }
    void write(T* p) { entries.push_back(p); }
};
struct PagingQ { Queue<NewPagingEntry> mPageQ; void addPage(NewPagingEntry* p) { mPageQ.write(p); } } gPagingQ;
static std::deque<NewPagingEntry*> ccchQueue;
static void pagerAddCcchMessageForGprs(NewPagingEntry* p) { ccchQueue.push_back(p); }
namespace GPRS {
#include "tlli-method.inc"
#include "assignment-start.inc"
#include "assignment-send.inc"
}
static const int L3_UNIT_DATA=0;
namespace GSM {
struct L2LogicalChannelBase {
    static void l2sendm(const L3PagingRequestType1&,int) { ++gsmSends; }
};
struct CCCHLogicalChannel {
    Time mCcchNextWriteTime=Time(1300); unsigned gprsSends=0; bool reservation=true;
    bool sendGprsCcchMessage(NewPagingEntry* p,Time& t) {
        assert(p->mGprsClient && p->mImmAssign && t.FN()==1352);
        assert(p->mImmAssign->packet.tlli==p->mGprsClient->mtGetTlli());
        assert(p->mImmAssign->packet.tfi==p->mGprsClient->mtTFI);
        if (!reservation) return false;
        ++gprsSends; return true;
    }
    bool processPages();
};
#include "process-pages.inc"
}
static void runGprs(const string& imsi,uint32_t tlli,bool reservation=true) {
    GPRS::MSInfo ms; ms.imsi=imsi; ms.handle=tlli;
    GPRS::TBF tbf; tbf.mtMS=&ms; tbf.mtTlli=tlli;
    GPRS::PDCHL1FEC pacch;
    const int lookups=gsmIdentityLookups;
    GPRS::sendAssignmentCcch(&pacch,&tbf,NULL);
    if (ccchQueue.empty() && imsi.empty()) {
        std::cerr << "EXPECTED-NEGATIVE: original drops pre-IMSI assignment\n";
        std::exit(17);
    }
    assert(ccchQueue.size()==1 && tbf.acks==1);
    assert(tbf.mtAssignCounter==1 && tbf.mtCcchAssignCounter==1);
    NewPagingEntry* p=ccchQueue.front(); ccchQueue.pop_front();
    assert(p->mImsi==imsi && p->mDrxBegin==1234 && p->mGprsClient==&tbf);
    assert(p->mImmAssign->packet.tlli==tbf.mtGetTlli() && aliveAssignments==1);
    // ccchServiceQueue's unchanged DRX branch transfers the entry to this FIFO.
    gPagingQ.addPage(p);
    CCCHLogicalChannel channel; channel.reservation=reservation;
    if (reservation) {
        assert(channel.processPages() && channel.processPages());
        assert(channel.gprsSends==2);
    } else {
        assert(!channel.processPages() && channel.gprsSends==0);
    }
    assert(!channel.processPages() && aliveAssignments==0);
    assert(gsmIdentityLookups==lookups && gPagingQ.mPageQ.entries.empty());
}
int main(int argc,char** argv) {
    if (argc==2 && string(argv[1])=="known-only") {
        runGprs("001010000000001",0xc0000001);
        std::cout << "PASS: original known-IMSI assignment remains functional\n";
        return 0;
    }
    runGprs("",0x80000001); // Foreign TLLI, initial identity exchange.
    runGprs("",0xc0000001); // Local TLLI without a surviving SGSN context.
    runGprs("001010000000001",0xc0000001); // Known PS identity, unchanged.
    runGprs("",0x80000001,false); // Reservation failure frees ownership.
    GPRS::gFixConvertForeignTLLI=true;
    runGprs("",0x80000001); // Existing explicit conversion switch is preserved.
    GPRS::gFixConvertForeignTLLI=false;
    string imsi="001010000000002";
    gPagingQ.addPage(new NewPagingEntry(SDCCHType,imsi));
    CCCHLogicalChannel channel;
    assert(channel.processPages() && channel.processPages() && !channel.processPages());
    assert(gsmIdentityLookups==2 && gsmSends==2 && channel.gprsSends==0);
    std::cout << "PASS: pre-IMSI/known-IMSI TLLI assignments, actual ctor, retries, ownership and CS paging\n";
}
