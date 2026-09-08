// Compiles production UHD receive methods extracted by test-uhd-rx-timeout.sh.
// The adapters model only UHD metadata/streaming; no device, service, or RF runs.
#include <algorithm>
#include <chrono>
#include <cstdint>
#include <cstring>
#include <iostream>
#include <sstream>
#include <stdexcept>
#include <string>
#include <sys/types.h>
#include <thread>
#include <utility>
#include <vector>

#ifndef EXPECT_PATCHED
#error EXPECT_PATCHED must be 0 or 1
#endif

using TIMESTAMP = unsigned long long;

namespace uhd {
class time_spec_t {
public:
    time_spec_t(long long ticks = 0, double = 0.0) : ticks_(ticks) {}
    static time_spec_t from_ticks(TIMESTAMP ticks, double) {
        return time_spec_t(static_cast<long long>(ticks));
    }
    TIMESTAMP to_ticks(double) const { return static_cast<TIMESTAMP>(ticks_); }
    double get_real_secs() const { return static_cast<double>(ticks_); }
    bool operator<(const time_spec_t &other) const { return ticks_ < other.ticks_; }
private:
    long long ticks_;
};

inline std::ostream &operator<<(std::ostream &out, const time_spec_t &ts) {
    return out << ts.get_real_secs();
}

struct rx_metadata_t {
    enum error_code_t {
        ERROR_CODE_NONE,
        ERROR_CODE_TIMEOUT,
        ERROR_CODE_OVERFLOW,
        ERROR_CODE_LATE_COMMAND,
        ERROR_CODE_BROKEN_CHAIN,
        ERROR_CODE_BAD_PACKET,
    };
    error_code_t error_code = ERROR_CODE_NONE;
    bool has_time_spec = false;
    time_spec_t time_spec;
};

inline std::string get_version_string() { return "test-uhd"; }
} // namespace uhd

static std::vector<std::string> log_lines;
struct LogLine {
    explicit LogLine(const char *level) { text << level << ": "; }
    ~LogLine() { log_lines.push_back(text.str()); }
    template <class T> LogLine &operator<<(const T &value) { text << value; return *this; }
    std::ostringstream text;
};
#define LOG(level) LogLine(#level)

#include "smpl-buf-class.inc"

struct FakePacket {
    std::vector<uint32_t> samples;
    uhd::rx_metadata_t metadata;
    unsigned delay_ms = 0;

    static FakePacket data(TIMESTAMP tick, std::initializer_list<uint32_t> values,
                           bool has_time = true) {
        FakePacket packet;
        packet.samples.assign(values.begin(), values.end());
        packet.metadata.error_code = uhd::rx_metadata_t::ERROR_CODE_NONE;
        packet.metadata.has_time_spec = has_time;
        packet.metadata.time_spec = uhd::time_spec_t::from_ticks(tick, 1.0);
        return packet;
    }
    static FakePacket empty(uhd::rx_metadata_t::error_code_t code,
                            unsigned delay = 0) {
        FakePacket packet;
        packet.metadata.error_code = code;
        packet.delay_ms = delay;
        return packet;
    }
};

class FakeRxStream {
public:
    explicit FakeRxStream(std::vector<FakePacket> input) : packets(std::move(input)) {}
    size_t recv(void *output, size_t capacity, uhd::rx_metadata_t &metadata,
                double, bool) {
        ++calls;
        FakePacket packet = next < packets.size()
            ? packets[next++]
            : FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT);
        if (packet.delay_ms)
            std::this_thread::sleep_for(std::chrono::milliseconds(packet.delay_ms));
        metadata = packet.metadata;
        size_t count = std::min(capacity, packet.samples.size());
        if (count)
            std::memcpy(output, packet.samples.data(), count * sizeof(uint32_t));
        return count;
    }

    std::vector<FakePacket> packets;
    size_t next = 0;
    size_t calls = 0;
};

class uhd_device {
public:
    enum err_code {
        ERROR_TIMING = -1,
        ERROR_UNRECOVERABLE = -2,
        ERROR_UNHANDLED = -3,
#if EXPECT_PATCHED
        ERROR_TIMEOUT = -4,
#endif
    };

    explicit uhd_device(std::vector<FakePacket> packets, size_t buffer_len = 64)
        : rx_stream(new FakeRxStream(std::move(packets))), rx_smpl_buf(new smpl_buf(buffer_len, 1.0)) {}
    ~uhd_device() { delete rx_stream; delete rx_smpl_buf; }

    int readSamples(short *buf, int len, bool *overrun, TIMESTAMP timestamp,
                    bool *underrun, unsigned *RSSI);
#if EXPECT_PATCHED
    int check_rx_md_err(uhd::rx_metadata_t &md, ssize_t num_smpls,
                        bool require_contiguous, TIMESTAMP requested_ts);
#else
    int check_rx_md_err(uhd::rx_metadata_t &md, ssize_t num_smpls);
#endif
    void restart(uhd::time_spec_t) {
        ++restart_calls;
#if EXPECT_PATCHED
        rx_pkt_end_valid = false;
#endif
    }
    std::string str_code(uhd::rx_metadata_t metadata) {
        return "metadata error " + std::to_string(metadata.error_code);
    }

    bool skip_rx = false;
    size_t rx_spp = 16;
    double rx_rate = 1.0;
    TIMESTAMP ts_offset = 0;
    size_t rx_pkt_cnt = 0;
    uhd::time_spec_t prev_ts;
#if EXPECT_PATCHED
    TIMESTAMP rx_pkt_end_ts = 0;
    bool rx_pkt_end_valid = false;
#endif
    FakeRxStream *rx_stream;
    smpl_buf *rx_smpl_buf;
    unsigned restart_calls = 0;
};

struct ExitCalled { int code; };
[[noreturn]] static void test_exit(int code) { throw ExitCalled{code}; }
#define exit(code) test_exit(code)
#include "uhd-rx-methods.inc"
#undef exit
#include "smpl-buf-methods.inc"

#if EXPECT_PATCHED
static unsigned checks = 0;
static void require(bool condition, const char *label) {
    if (!condition) {
        std::cerr << "FAIL: " << label << '\n';
        throw std::runtime_error(label);
    }
    ++checks;
}

static size_t count_log(const std::string &needle) {
    size_t count = 0;
    for (const std::string &line : log_lines)
        if (line.find(needle) != std::string::npos) ++count;
    return count;
}
#endif

static int read(uhd_device &device, std::vector<uint32_t> &output,
                int length, TIMESTAMP timestamp, bool &exited) {
    bool overrun = false, underrun = false;
    unsigned rssi = 0;
    try {
        return device.readSamples(reinterpret_cast<short *>(output.data()), length,
                                  &overrun, timestamp, &underrun, &rssi);
    } catch (const ExitCalled &) {
        exited = true;
        return -1;
    }
}

#if EXPECT_PATCHED
static void test_transient_and_data_integrity() {
    log_lines.clear();
    uhd_device device({
        FakePacket::data(0, {10, 11, 12, 13}),
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::data(4, {14, 15, 16, 17}),
    });
    std::vector<uint32_t> output(8, 0xdeadbeef);
    bool exited = false;
    require(read(device, output, 8, 0, exited) == 8 && !exited,
            "transient timeout recovers");
    require(output == std::vector<uint32_t>({10,11,12,13,14,15,16,17}),
            "recovery neither fabricates nor corrupts samples");
    require(device.restart_calls == 0, "timeout recovery does not restart RF clock");
    require(count_log("Receive timed out; waiting") == 1,
            "timeout streak logs once rather than every wait");
    require(count_log("Receive recovered after 2 timeout(s)") == 1,
            "recovery log reports timeout count");
}

static void test_counter_resets_only_after_valid_data() {
    std::vector<FakePacket> packets;
    for (unsigned i = 0; i < 6; ++i)
        packets.push_back(FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT));
    packets.push_back(FakePacket::data(0, {1,2,3,4}));
    for (unsigned i = 0; i < 6; ++i)
        packets.push_back(FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT));
    packets.push_back(FakePacket::data(4, {5,6,7,8}));
    uhd_device device(std::move(packets));
    std::vector<uint32_t> first(4), second(4);
    bool exited = false;
    require(read(device, first, 4, 0, exited) == 4 && !exited,
            "first sub-limit timeout streak recovers");
    require(read(device, second, 4, 4, exited) == 4 && !exited,
            "valid data resets timeout count for next read");
    require(first == std::vector<uint32_t>({1,2,3,4}) &&
            second == std::vector<uint32_t>({5,6,7,8}),
            "reset path preserves both sample blocks");
}

static void test_timeout_count_is_bounded() {
    std::vector<FakePacket> packets;
    for (unsigned i = 0; i < 10; ++i)
        packets.push_back(FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT));
    uhd_device device(std::move(packets));
    std::vector<uint32_t> output(1, 0xa5a5a5a5);
    bool exited = false;
    require(read(device, output, 1, 0, exited) == -1 && exited,
            "ten consecutive timeouts are terminal");
    require(device.rx_stream->calls == 10, "retry count is exactly bounded at ten");
    require(device.restart_calls == 0, "permanent timeout does not restart device");
    require(output[0] == 0xa5a5a5a5, "terminal timeout does not write caller data");
}

static void test_unhandled_does_not_reset_budget() {
    std::vector<FakePacket> packets;
    for (unsigned i = 0; i < 9; ++i) {
        packets.push_back(FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT));
        packets.push_back(FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_BAD_PACKET));
    }
    packets.push_back(FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT));
    uhd_device device(std::move(packets));
    std::vector<uint32_t> output(1);
    bool exited = false;
    read(device, output, 1, 0, exited);
    require(exited && device.rx_stream->calls == 19,
            "unhandled metadata cannot reset the timeout count");
}

static void test_wall_clock_deadline_crosses_unhandled_error() {
    uhd_device device({
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_BAD_PACKET, 1050),
        FakePacket::data(0, {7}),
    });
    std::vector<uint32_t> output(1);
    bool exited = false;
    read(device, output, 1, 0, exited);
    require(exited && device.rx_stream->calls == 2,
            "one-second deadline applies while errors interrupt timeout waits");
}

static void expect_discontinuity(TIMESTAMP resumed_tick, const char *label) {
    uhd_device device({
        FakePacket::data(0, {1,2,3,4}),
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::data(resumed_tick, {5,6,7,8}),
    });
    std::vector<uint32_t> output(8, 0xcccccccc);
    bool exited = false;
    read(device, output, 8, 0, exited);
    require(exited, label);
    require(device.restart_calls == 0, "timeout discontinuity is not auto-restarted");
    require(std::all_of(output.begin(), output.end(), [](uint32_t value) {
                return value == 0xcccccccc;
            }), "discontinuity does not expose buffered or stale samples");
}

static void test_continuity_guards() {
    expect_discontinuity(5, "forward timestamp gap after timeout is terminal");
    expect_discontinuity(3, "overlapping timestamp after timeout is terminal");

    uhd_device startup({
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::data(6, {1,2,3,4}),
    });
    std::vector<uint32_t> output(4, 0xeeeeeeee);
    bool exited = false;
    read(startup, output, 4, 4, exited);
    require(exited, "late first packet after startup timeout is terminal");

    uhd_device covering({
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::data(0, {10,11,12,13,14,15,16,17}),
    });
    std::vector<uint32_t> covered(4);
    exited = false;
    require(read(covering, covered, 4, 4, exited) == 4 && !exited,
            "first resumed packet may precede and cover requested tick");
    require(covered == std::vector<uint32_t>({14,15,16,17}),
            "covered startup read selects correct samples");

    uhd_device backward({
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::data(0, {1}),
    });
    backward.prev_ts = uhd::time_spec_t::from_ticks(10, 1.0);
    std::vector<uint32_t> one(1);
    exited = false;
    read(backward, one, 1, 0, exited);
    require(exited && backward.restart_calls == 0,
            "backward recovery after timeout is fatal without restart");
}

static void test_metadata_semantics() {
    uhd_device missing({FakePacket::data(0, {1}, false)});
    std::vector<uint32_t> output(1);
    bool exited = false;
    read(missing, output, 1, 0, exited);
    require(exited, "missing timestamp remains fatal");

    uhd_device unhandled({
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_BAD_PACKET),
        FakePacket::data(0, {9}),
    });
    exited = false;
    require(read(unhandled, output, 1, 0, exited) == 1 && !exited && output[0] == 9,
            "legacy non-timeout unhandled metadata still continues");

    uhd_device timing({});
    timing.prev_ts = uhd::time_spec_t::from_ticks(10, 1.0);
    uhd::rx_metadata_t metadata;
    metadata.has_time_spec = true;
    metadata.time_spec = uhd::time_spec_t::from_ticks(9, 1.0);
    require(timing.check_rx_md_err(metadata, 1, false, 0) == uhd_device::ERROR_TIMING,
            "ordinary backward metadata keeps legacy timing result");
}

static void test_actual_sample_buffer_wrap() {
    smpl_buf buffer(8, 1.0);
    uint32_t first[] = {0,1,2,3,4,5};
    uint32_t second[] = {6,7,8,9,10};
    uint32_t output[7] = {};
    require(buffer.write(first, 6, static_cast<TIMESTAMP>(0)) == 6,
            "sample buffer initial partial write");
    uint32_t discarded[4] = {};
    require(buffer.read(discarded, 4, static_cast<TIMESTAMP>(0)) == 4,
            "sample buffer partial read advances ring");
    require(buffer.write(second, 5, static_cast<TIMESTAMP>(6)) == 5,
            "sample buffer write wraps ring");
    require(buffer.read(output, 7, static_cast<TIMESTAMP>(4)) == 7,
            "sample buffer reads wrapped range");
    require(std::vector<uint32_t>(output, output + 7) ==
            std::vector<uint32_t>({4,5,6,7,8,9,10}),
            "wrapped sample data remains ordered and intact");
}

int main() {
    try {
        test_transient_and_data_integrity();
        test_counter_resets_only_after_valid_data();
        test_timeout_count_is_bounded();
        test_unhandled_does_not_reset_budget();
        test_wall_clock_deadline_crosses_unhandled_error();
        test_continuity_guards();
        test_metadata_semantics();
        test_actual_sample_buffer_wrap();
    } catch (const std::exception &) {
        return 1;
    }
    std::cout << "PASS: extracted UHD RX methods and sample buffer, "
              << checks << " checks (no RF)\n";
    return 0;
}
#else
int main() {
    uhd_device device({
        FakePacket::empty(uhd::rx_metadata_t::ERROR_CODE_TIMEOUT),
        FakePacket::data(0, {1}),
    });
    std::vector<uint32_t> output(1);
    bool exited = false;
    read(device, output, 1, 0, exited);
    if (exited && device.rx_stream->calls == 1) {
        std::cerr << "EXPECTED-NEGATIVE: original exits on first transient timeout\n";
        return 1;
    }
    std::cerr << "NEGATIVE CONTROL DID NOT REPRODUCE ORIGINAL FAILURE\n";
    return 0;
}
#endif
