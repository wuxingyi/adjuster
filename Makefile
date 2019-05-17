#!/usr/bin/make -f

all: adjuster

adjuster: adjuster.go
	go build adjuster.go

install: adjuster
	mkdir -p $(DESTDIR)/lib/systemd/system
	mkdir -p $(DESTDIR)/var/log/adjuster
	install -m 0755 -D adjuster $(DESTDIR)/usr/bin/adjuster
	install -m 0644 -D adjuster.service $(DESTDIR)/lib/systemd/system/adjuster.service
	install -m 0644 -D logrotater_adjuster $(DESTDIR)/etc/logrotate.d/logrotater_adjuster

clean:
	rm -f adjuster

distclean: clean

uninstall:
	rm -f $(DESTDIR)/usr/bin/adjuster
	rm -f $(DESTDIR)/var/log/adjuster
	rm -f $(DESTDIR)/lib/systemd/system/adjuster.service

.PHONY: all install clean distclean uninstall
