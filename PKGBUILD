# Maintainer: codechenx <codechenx@gmail.com>
pkgname=ftv-bin
_pkgname=ftv
pkgver=0.9.2
pkgrel=1
pkgdesc="A fast, feature-rich CSV/TSV/delimited file viewer for the command line"
arch=('x86_64')
url="https://github.com/codechenx/FastTableViewer"
license=('Apache')
provides=("${_pkgname}")
conflicts=("${_pkgname}")
source=("${url}/releases/download/v${pkgver}/${_pkgname}_${pkgver}_Linux_x86_64.tar.gz")
sha256sums=('SKIP')

package() {
    install -Dm755 "${_pkgname}" "${pkgdir}/usr/bin/${_pkgname}"
    install -Dm644 LICENSE "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"
    install -Dm644 README.md "${pkgdir}/usr/share/doc/${pkgname}/README.md"
}
