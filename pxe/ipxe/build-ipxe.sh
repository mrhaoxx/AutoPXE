#!/bin/bash -xe

rm -rf ipxe
proxychains git clone https://github.com/ipxe/ipxe.git --depth=1

cd ipxe/src

make bin/undionly.kpxe EMBED=../../embed.ipxe -j
make bin-x86_64-efi/snponly.efi EMBED=../../embed.ipxe -j

cp bin/undionly.kpxe ../../
cp bin-x86_64-efi/snponly.efi ../../

cd ../..
rm -rf ipxe
