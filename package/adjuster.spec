%global debug_package %{nil}
%global __strip /bin/true

Name:           adjuster
Version:        %{ver}
Release:        %{rel}%{?dist}

Summary:	adjuster

Group:		SDS
License:	GPL
URL:		http://github.com/wuxingyi/adjuster
Source0:	%{name}-%{version}-%{rel}.tar.gz
BuildRoot:	%(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-XXXXXX)

%description
adjuster for bcache writeback rate


%prep
%setup -q -n %{name}-%{version}-%{rel}

%build
make

%install
mkdir -p %{buildroot}/lib/systemd/system
mkdir -p %{buildroot}/var/log/adjuster
install -m 0755 -D adjuster %{buildroot}/usr/bin/adjuster
install -m 0644 -D adjuster.service %{buildroot}/lib/systemd/system/adjuster.service
install -m 0644 -D logrotater_adjuster %{buildroot}/etc/logrotate.d/logrotater_adjuster
#mkdir -p  %{buildroot}/usr/bin
#cp adjuster    %{buildroot}/usr/bin/adjuster
#cp adjuster.service    %{buildroot}/etc/systemd/system/adjuster.service

#ceph confs ?

%post


%preun

%clean
rm -rf %{buildroot}

%files
%defattr(-,root,root,-)
/usr/bin/adjuster
/lib/systemd/system/adjuster.service
/etc/logrotate.d/logrotater_adjuster

%changelog
