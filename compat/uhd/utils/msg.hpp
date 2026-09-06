#pragma once
// Compat shim: UHD >= 4.0 removed uhd::msg (message-task log routing).
// OpenBTS 5.0 uses it in exactly one place (Transceiver52M/UHDDevice.cpp):
//   uhd::msg::register_handler(&uhd_msg_handler);
//   void uhd_msg_handler(uhd::msg::type_t, const std::string &)
// UHD 4.x logs to stderr natively, so the handler is accepted and stored
// but never invoked by the driver. Build openbts with -I/app/compat so
// this header (absent upstream) resolves; nothing else is shadowed.
#include <string>

namespace uhd { namespace msg {

enum type_t {
    status   = 0,
    warning  = 1,
    error    = 2,
    fastpath = 3
};

typedef void (*handler_t)(type_t, const std::string &);

namespace detail {
inline handler_t &slot() {
    static handler_t h = 0;
    return h;
}
} // namespace detail

inline void register_handler(handler_t h) { detail::slot() = h; }

}} // namespace uhd::msg
