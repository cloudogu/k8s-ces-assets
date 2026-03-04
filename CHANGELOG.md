# k8s-ces-assets Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v2.0.1] - 2026-03-04
### Changed
- [#19] Allow k8s-ces-gateway in version range 3.x.x 

## [v2.0.0] - 2026-02-26
> [!IMPORTANT]
> Breaking change!
> New compatible versions of k8s-ces-service-discovery, k8s-backup-operator and ces-exporter are required.

### Changed
- [#15] Use the new maintenance ConfigMap
  - Previously, the maintenance mode was read from the global config

## [v1.0.5] - 2026-02-17
### Security
- [#17] Fix Go stdlib CVE-2025-68121

## [v1.0.4] - 2025-11-27
### Changed
- [#11] define start order after ces-gateway

## [v1.0.3] - 2025-10-24
### Fixed
- [#9] warp menu doesn't contain dogus because of an error while watching config maps

## [v1.0.2] - 2025-10-01
### Fixed
- [#7] images in component-patch-template

## [v1.0.1] - 2025-09-29
### Fixed
- [#5] missing network-policy

## [v1.0.0] - 2025-09-23
### Added
- [#4] documentation for asset usecases

## [v0.1.0] - 2025-09-12
### Added
- [#1] initial project structure
- [#2] warp-menu generation
- [#3] maintenancemode handling