import base64
import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import bundle_backend
import install_release as installer
import sync_prices


def archive(entries):
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode='w:gz') as tar:
        for name, data, kind in entries:
            member = tarfile.TarInfo(name)
            member.type = kind
            member.size = len(data)
            member.mode = 0o755
            tar.addfile(member, io.BytesIO(data))
    return buffer.getvalue()


class InstallerTests(unittest.TestCase):
    def test_checksum_rejects_tampering_and_ambiguity(self):
        data = b'valid archive';digest = hashlib.sha256(data).hexdigest()
        self.assertEqual(installer.verify_checksum(data, 'app.tgz', f'{digest}  app.tgz\n'.encode()), digest)
        for checksums in [b'', f'{digest} other.tgz'.encode(), f'{digest} app.tgz\n{digest} app.tgz'.encode()]:
            with self.assertRaises(ValueError):
                installer.verify_checksum(data, 'app.tgz', checksums)
        with self.assertRaises(ValueError):
            installer.verify_checksum(b'tampered', 'app.tgz', f'{digest} app.tgz'.encode())

    def test_official_asset_validation(self):
        release = {'tag_name':'v0.3.0','draft':False,'prerelease':False,'assets':[]}
        for name in ['tokenlens_0.3.0_darwin_arm64.tar.gz','tokenlens_0.3.0_checksums.txt']:
            release['assets'].append({'name':name,'browser_download_url':f'https://github.com/Kameleon21/tokenlens/releases/download/v0.3.0/{name}'})
        self.assertEqual(installer.release_assets(release,'darwin_arm64')[0],'v0.3.0')
        release['assets'][0]['browser_download_url']='https://example.com/other'
        with self.assertRaises(ValueError):installer.release_assets(release,'darwin_arm64')

    def test_safe_extraction_keeps_bundle_and_rejects_links(self):
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory)
            data = archive([('tokenlens',b'app',tarfile.REGTYPE),('libexec/ccusage',b'backend',tarfile.REGTYPE)])
            installer.extract_archive(data,destination)
            self.assertEqual((destination/'libexec/ccusage').read_bytes(),b'backend')
            for name,kind in [('../escape',tarfile.REGTYPE),('/absolute',tarfile.REGTYPE),('symlink',tarfile.SYMTYPE),('hardlink',tarfile.LNKTYPE)]:
                with self.assertRaises(ValueError):installer.extract_archive(archive([(name,b'',kind)]),destination)

    def test_failed_version_check_preserves_installed_executable(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory);bin_dir=root/'bin';bin_dir.mkdir();old=bin_dir/'tokenlens';old.write_text('existing')
            data=archive([('tokenlens',b'new',tarfile.REGTYPE)])
            with patch.object(installer.subprocess,'check_output',return_value='tokenlens 9.9.9'), self.assertRaises(ValueError):
                installer.install_verified(data,'v0.3.0','darwin_arm64','a'*64,root/'data',bin_dir)
            self.assertEqual(old.read_text(),'existing')

    def test_install_switches_complete_bundle_and_preserves_old_binary(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);bin_dir=root/'bin';bin_dir.mkdir();(bin_dir/'tokenlens').write_text('existing')
            data=archive([('tokenlens',b'new',tarfile.REGTYPE),('libexec/ccusage',b'backend',tarfile.REGTYPE),('scripts/install_release.py',b'updater',tarfile.REGTYPE)])
            with patch.object(installer.subprocess,'check_output',return_value='tokenlens 0.3.0'):
                installed=installer.install_verified(data,'v0.3.0','darwin_arm64','a'*64,root/'data',bin_dir)
            self.assertTrue(installed.is_symlink())
            self.assertEqual((installed.resolve().parent/'libexec/ccusage').read_bytes(),b'backend')
            self.assertEqual(next(bin_dir.glob('tokenlens-before-*')).read_text(),'existing')
            self.assertEqual((root/'data/update.py').read_text(),'updater')

    def test_native_package_integrity_and_member_type(self):
        data=archive([('package/bin/ccusage',b'native',tarfile.REGTYPE)])
        integrity='sha512-'+base64.b64encode(hashlib.sha512(data).digest()).decode()
        self.assertEqual(bundle_backend.verified_binary(data,integrity,'ccusage'),b'native')
        with self.assertRaises(ValueError):bundle_backend.verified_binary(data+b'changed',integrity,'ccusage')
        data=archive([('package/bin/ccusage',b'',tarfile.SYMTYPE)])
        integrity='sha512-'+base64.b64encode(hashlib.sha512(data).digest()).decode()
        with self.assertRaises(ValueError):bundle_backend.verified_binary(data,integrity,'ccusage')

    def test_catalog_normalization_and_embedded_snapshot(self):
        model={'input_cost_per_token':1,'output_cost_per_token':2,'provider_specific_entry':{'fast':2},'input_cost_per_token_above_200k_tokens':3}
        prices=sync_prices.normalize({'known':model,'invalid':{'input_cost_per_token':-1,'output_cost_per_token':2}})
        self.assertEqual(list(prices),['known'])
        self.assertEqual(prices['known']['fastMultiplier'],2)
        self.assertEqual(prices['known']['inputCostPerTokenAbove200kTokens'],3)
        snapshot=json.loads((sync_prices.ROOT/'internal/app/pricedata/litellm.json').read_text())
        self.assertEqual(snapshot['version'],1)
        self.assertGreater(len(snapshot['models']),100)


if __name__=='__main__':unittest.main()
