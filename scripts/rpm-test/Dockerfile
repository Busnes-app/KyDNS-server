# The base images carry no systemd, and every claim this package makes is a
# systemd claim — enabled or not, running as kydns, bound to a privileged
# port, sandboxed by the unit's directives. So the test host installs one.
ARG BASE=fedora:44@sha256:43b29f65a41eb9c35e1cd5323e3bdf3b655c2357a9f4f1ff2f9c2798e5045d80
FROM ${BASE}

# --allowerasing: Rocky's minimal image ships curl-minimal, which conflicts
# with the full curl this suite needs for -sf. dnf5 (Fedora) only accepts
# the flag after the subcommand; dnf4 (Rocky) accepts it there too.
RUN dnf -y install --allowerasing systemd iproute bind-utils curl procps-ng \
	&& dnf clean all

# Nothing in a test container wants a getty, and remounting / fails in one.
RUN systemctl mask getty.target systemd-remount-fs.service

# systemd's shutdown signal, so `docker stop` is not a 10-second wait.
STOPSIGNAL SIGRTMIN+3
CMD ["/usr/sbin/init"]
