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
mkdir -p  %{buildroot}/usr/bin
cp adjuster    %{buildroot}/usr/bin/adjuster

#ceph confs ?

%post


%preun

%clean
rm -rf %{buildroot}

%files
%defattr(-,root,root,-)
/usr/bin/adjuster

%changelog
