# SPDX-License-Identifier: FSL-1.1-MIT

FROM ubuntu:24.04

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates curl dbus libpam-systemd openssh-server systemd systemd-sysv \
    && useradd -u 1500 -m -s /bin/bash pedro \
    && echo 'pedro:ssh-fixture-correct' | chpasswd \
    && systemctl mask ssh.service ssh.socket systemd-resolved.service \
    && rm -f /etc/ssh/ssh_host_* \
    && rm -rf /var/lib/apt/lists/*

COPY sshd_config /etc/ssh/compozy_fixture_config
COPY --chmod=755 command.sh /usr/local/bin/fixture-ssh-command

ENV container=docker
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
