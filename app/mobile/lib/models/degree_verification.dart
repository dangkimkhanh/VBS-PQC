/// Response of `GET /api/v1/public/degrees/:id`, the same data the web
/// verification page shows.
class DegreeVerification {
  final String id;
  final String fullName;
  final String universityName;
  final String name;
  final String certificateType;
  final DateTime? issueDate;
  final String serialNumber;
  final bool revoked;
  final DateTime? revokedAt;
  final String fileHash;
  final VerificationResult verification;

  const DegreeVerification({
    required this.id,
    required this.fullName,
    required this.universityName,
    required this.name,
    required this.certificateType,
    required this.issueDate,
    required this.serialNumber,
    required this.revoked,
    required this.revokedAt,
    required this.fileHash,
    required this.verification,
  });

  factory DegreeVerification.fromJson(Map<String, dynamic> json) {
    return DegreeVerification(
      id: '${json['id'] ?? ''}',
      fullName: '${json['full_name'] ?? ''}',
      universityName: '${json['university_name'] ?? ''}',
      name: '${json['name'] ?? ''}',
      certificateType: '${json['certificate_type'] ?? ''}',
      issueDate: _date(json['issue_date']),
      serialNumber: '${json['serial_number'] ?? ''}',
      revoked: json['revoked'] == true,
      revokedAt: _date(json['revoked_at']),
      fileHash: '${json['file_hash'] ?? ''}'.toLowerCase(),
      verification: VerificationResult.fromJson(
        (json['verification'] as Map?)?.cast<String, dynamic>() ?? const {},
      ),
    );
  }

  /// Every layer passed and the diploma is not revoked.
  bool get complete => !revoked && verification.valid;

  /// Signature and PDF are valid but the blockchain proof is not confirmed yet.
  bool get signedOnly =>
      !revoked && verification.signatureValid && verification.fileIntegrityValid;
}

class VerificationResult {
  final bool valid;
  final bool signatureValid;
  final bool fileIntegrityChecked;
  final bool fileIntegrityValid;
  final String algorithm;
  final String assuranceLevel;
  final BlockchainResult blockchain;
  final String warning;
  final String error;

  const VerificationResult({
    required this.valid,
    required this.signatureValid,
    required this.fileIntegrityChecked,
    required this.fileIntegrityValid,
    required this.algorithm,
    required this.assuranceLevel,
    required this.blockchain,
    required this.warning,
    required this.error,
  });

  factory VerificationResult.fromJson(Map<String, dynamic> json) {
    return VerificationResult(
      valid: json['valid'] == true,
      signatureValid: json['signature_valid'] == true,
      fileIntegrityChecked: json['file_integrity_checked'] == true,
      fileIntegrityValid: json['file_integrity_valid'] == true,
      algorithm: '${json['algorithm'] ?? ''}',
      assuranceLevel: '${json['assurance_level'] ?? ''}',
      blockchain: BlockchainResult.fromJson(
        (json['blockchain'] as Map?)?.cast<String, dynamic>() ?? const {},
      ),
      warning: '${json['warning'] ?? ''}',
      error: '${json['error'] ?? ''}',
    );
  }

  String get algorithmLabel => algorithm.isEmpty ? 'ML-DSA' : algorithm;
}

class BlockchainResult {
  final String status;
  final bool valid;
  final String batchId;
  final String transactionId;

  const BlockchainResult({
    required this.status,
    required this.valid,
    required this.batchId,
    required this.transactionId,
  });

  factory BlockchainResult.fromJson(Map<String, dynamic> json) {
    return BlockchainResult(
      status: '${json['status'] ?? ''}',
      valid: json['valid'] == true,
      batchId: '${json['batch_id'] ?? ''}',
      transactionId: '${json['transaction_id'] ?? ''}',
    );
  }

  static const _labels = {
    'verified': 'Đã xác nhận',
    'not_anchored': 'Chưa ghi Blockchain',
    'not_checked': 'Chưa kiểm tra',
    'unavailable': 'Không kết nối được Blockchain',
    'mismatch': 'Không khớp dữ liệu trên Blockchain',
    'inconsistent': 'Dữ liệu neo không nhất quán',
    'invalid_local_proof': 'Bằng chứng Merkle không hợp lệ',
    'missing_pqc_transaction': 'Thiếu chữ ký giao dịch PQC',
    'legacy_batch': 'Lô ghi theo định dạng cũ',
  };

  String get label => _labels[status] ?? (status.isEmpty ? 'Không xác định' : status);
}

DateTime? _date(dynamic value) {
  if (value == null) return null;
  final parsed = DateTime.tryParse('$value');
  // Go encodes an unset time as 0001-01-01.
  if (parsed == null || parsed.year < 1900) return null;
  return parsed.toLocal();
}

/// dd/MM/yyyy, as on the web page.
String formatDate(DateTime? date) {
  if (date == null) return '—';
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(date.day)}/${two(date.month)}/${date.year}';
}
