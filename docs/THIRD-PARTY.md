# Third-party distribution / 第三方分发说明

The project MIT license covers project-owned code only. Native OpenBTS, smqueue,
subscriberRegistry, liba53, libcoredumper, UHD, cppzmq, Asterisk, Ubuntu packages
and firmware retain their own terms. This document does not certify rights.
项目 MIT 仅适用于项目自有代码；上述原生组件、系统包和固件保留各自条款，本文不是授权认证。

## Exact source and notices / 对应源码与许可文件

- The release source archive contains the pinned `third_party` trees (including
  initialized recursive submodules and the coredumper archive), local patches,
  build scripts and clean seeds. `VENDOR-REVISIONS.tsv` records revisions/origins;
  `SOURCE-REVISION` links the project commit. Release assets include its SHA-256.
  发布源码包含固定上游源码及递归子模块、coredumper 源码归档、本地补丁、构建脚本和干净种子，
  清单记录版本及来源，并附项目 revision 和 SHA-256。
- Final images include discovered native upstream LICENSE/COPYING/NOTICE/COPYRIGHT
  files under `/usr/share/doc/gsm-system/upstream-notices`, plus this document and
  the project license. Distribution-package copyright files remain in
  `/usr/share/doc`. The source archive retains the complete source trees and their
  original notices, not just these selected notice filenames.
  镜像保留原生源码中的许可/版权文件及本文；系统包版权文件保留在 `/usr/share/doc`。
  源码包同时保留完整源码及原始说明，不限于选取的 notice 文件名。
- Publish the corresponding source archive with each image release and keep it
  available alongside the binary. Do not label the entire image “MIT”. Validate
  exact component terms, modifications and any source-provision obligations before
  distribution; notice collection is not a legal-compliance certification.
  每次镜像发布附对应源码并持续提供；不要将整张镜像标为 MIT。分发前核对各组件条款、修改和
  源码提供义务，收集 notice 不等于合规认证。

## Bundled board firmware / 自带板卡固件

`firmware/uhd/README.md` describes the vendor-bundle origin of the BlackSDR FPGA
and the working UHD 4.x FX3 firmware set. Vendor redistribution terms were not
provided in this repository. The user's requested image publication retains these
existing hardware inputs; neither that request nor this provenance statement is a
vendor license grant. Maintainers remain responsible for applicable firmware terms.
固件来源见该 README；仓库未随附供应商再分发条款。本次依用户要求保留已有硬件输入，
用户发布请求和来源记录都不等同于供应商授权。维护者仍需核对适用固件条款。

## Data hygiene / 数据卫生

The publication gate checks empty subscriber/SIP/RRLP tables, allowlisted TMSI
schema-version metadata, firmware checksums and active credential-like Asterisk
directives without logging values. Docker and source packaging reconstruct seed
databases from schema plus the allowed VERSION=7 record; historical database pages
are never copied into final layers. Legacy archives and live volumes are excluded.
发布门禁检查业务空表、TMSI 版本元数据、固件哈希和 Asterisk 凭据样式指令，不打印值。
Docker 和源码包从 schema 及允许的 VERSION=7 元数据重建种子，不复制历史页；
排除旧数据库归档和现网数据卷。定向检查不替代完整内容审查。
