#!/usr/bin/make -f

all: adjuster

adjuster: adjuster.go
	go build adjuster.go

install: adjuster
	install -D adjuster $(DESTDIR)/usr/bin/adjuster
	mkdir -p $(DESTDIR)/lib/systemd/system
	mkdir -p $(DESTDIR)/var/log/adjuster
	install -D adjuster.service $(DESTDIR)/lib/systemd/system/adjuster.service

clean:
	rm -f adjuster

distclean: clean

uninstall:
	rm -f $(DESTDIR)/usr/bin/adjuster
	rm -f $(DESTDIR)/var/log/adjuster
	rm -f $(DESTDIR)/lib/systemd/system/adjuster.service

.PHONY: all install clean distclean uninstall
