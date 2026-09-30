import 'package:flutter_test/flutter_test.dart';
import 'package:vbs_pqc_verify/models/degree_verification.dart';
import 'package:vbs_pqc_verify/services/api_service.dart';

void main() {
  group('parseVerificationCode', () {
    const id = '6ab78bab51113319eae7ce92';

    test('accepts the code printed on the diploma', () {
      expect(ApiService.parseVerificationCode('  $id  '), id);
      expect(ApiService.parseVerificationCode(id.toUpperCase()), id);
    });

    test('accepts the verification link from the QR code', () {
      expect(ApiService.parseVerificationCode('https://54.79.163.176.nip.io/verify/$id'), id);
      expect(ApiService.parseVerificationCode('https://example.com/verify/$id?ref=qr'), id);
    });

    test('rejects anything else', () {
      expect(ApiService.parseVerificationCode(''), isNull);
      expect(ApiService.parseVerificationCode('https://example.com/'), isNull);
      expect(ApiService.parseVerificationCode('not-a-code'), isNull);
    });
  });

  test('reads the public verification response like the web page', () {
    final degree = DegreeVerification.fromJson({
      'id': '6ab78bab51113319eae7ce92',
      'full_name': 'Đặng Kim Khánh',
      'university_name': 'Học viện giao thông vận tải',
      'name': 'Bằng tốt nghiệp',
      'certificate_type': 'Kỹ sư',
      'issue_date': '2026-09-16T00:00:00Z',
      'serial_number': '444',
      'revoked': false,
      'revoked_at': null,
      'file_hash': 'ABCDEF',
      'verification': {
        'valid': false,
        'signature_valid': true,
        'file_integrity_checked': true,
        'file_integrity_valid': true,
        'algorithm': 'ML-DSA-65',
        'blockchain': {'status': 'not_anchored', 'valid': false},
      },
    });

    expect(degree.complete, isFalse);
    expect(degree.signedOnly, isTrue);
    expect(degree.fileHash, 'abcdef');
    expect(degree.verification.blockchain.label, 'Chưa ghi Blockchain');
    expect(formatDate(degree.issueDate), '16/09/2026');
  });

  test('treats an unset Go time as missing', () {
    expect(formatDate(DegreeVerification.fromJson({'issue_date': '0001-01-01T00:00:00Z'}).issueDate), '—');
  });
}
