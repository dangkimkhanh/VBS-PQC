import 'package:crypto/crypto.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';

import '../models/degree_verification.dart';
import '../services/api_service.dart';
import '../theme.dart';

/// Public verification result for one diploma, mirroring the web page
/// /verify/:id: overall verdict, diploma details, the three checks and an
/// optional comparison with a PDF the verifier holds.
class VerificationScreen extends StatefulWidget {
  final String degreeId;

  const VerificationScreen({super.key, required this.degreeId});

  @override
  State<VerificationScreen> createState() => _VerificationScreenState();
}

class _VerificationScreenState extends State<VerificationScreen> {
  late Future<DegreeVerification> _future;

  @override
  void initState() {
    super.initState();
    _future = ApiService.verifyDegree(widget.degreeId);
  }

  void _retry() => setState(() => _future = ApiService.verifyDegree(widget.degreeId));

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      appBar: AppBar(title: const Text('Kết quả xác minh')),
      body: FutureBuilder<DegreeVerification>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const _Loading();
          }
          if (snapshot.hasError) {
            return _ErrorView(message: '${snapshot.error}', onRetry: _retry);
          }
          final degree = snapshot.data!;
          return RefreshIndicator(
            onRefresh: () async {
              _retry();
              await _future.catchError((_) => degree);
            },
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
              children: [
                _VerdictBanner(degree: degree),
                const SizedBox(height: 16),
                _DegreeCard(degree: degree),
                const SizedBox(height: 16),
                _ChecksGrid(result: degree.verification),
                const SizedBox(height: 16),
                _FileCompareCard(expectedHash: degree.fileHash),
              ],
            ),
          );
        },
      ),
    );
  }
}

class _Loading extends StatelessWidget {
  const _Loading();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 16),
          Text('Đang kiểm tra chữ ký, tệp PDF và Blockchain…'),
        ],
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorView({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.warning_amber_rounded, size: 48, color: AppColors.warning),
            const SizedBox(height: 12),
            const Text('Không thể xác minh', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w600)),
            const SizedBox(height: 8),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton(onPressed: onRetry, child: const Text('Thử lại')),
          ],
        ),
      ),
    );
  }
}

enum _Tone { success, successSoft, danger, warning }

class _VerdictBanner extends StatelessWidget {
  final DegreeVerification degree;

  const _VerdictBanner({required this.degree});

  @override
  Widget build(BuildContext context) {
    final v = degree.verification;
    final _Tone tone;
    final String title;
    final String detail;
    if (degree.revoked) {
      tone = _Tone.danger;
      title = 'Văn bằng đã thu hồi';
      detail = 'Văn bằng này không còn hiệu lực, kể cả khi chữ ký gốc vẫn đúng.';
    } else if (degree.complete) {
      tone = _Tone.success;
      title = 'Xác minh thành công';
      detail = 'Chữ ký ${v.algorithmLabel}, tệp PDF và bằng chứng Blockchain đều khớp.';
    } else if (degree.signedOnly) {
      tone = _Tone.successSoft;
      title = 'Chữ ký và PDF hợp lệ';
      detail = 'Bằng chứng Blockchain chưa được xác nhận. Xem riêng từng kết quả bên dưới.';
    } else {
      tone = _Tone.warning;
      title = 'Chưa xác minh được văn bằng';
      detail = 'Kiểm tra các kết quả bên dưới hoặc liên hệ đơn vị cấp.';
    }

    final (Color bg, Color border, Color fg, IconData icon) = switch (tone) {
      _Tone.success => (const Color(0xFFD1FAE5), const Color(0xFF6EE7B7), const Color(0xFF064E3B), Icons.check_circle),
      _Tone.successSoft => (const Color(0xFFECFDF5), const Color(0xFFA7F3D0), const Color(0xFF064E3B), Icons.check_circle),
      _Tone.danger => (const Color(0xFFFFF1F2), const Color(0xFFFDA4AF), const Color(0xFF881337), Icons.cancel),
      _Tone.warning => (const Color(0xFFFFFBEB), const Color(0xFFFCD34D), const Color(0xFF78350F), Icons.warning_amber_rounded),
    };
    final iconColor = switch (tone) {
      _Tone.success || _Tone.successSoft => AppColors.success,
      _Tone.danger => AppColors.danger,
      _Tone.warning => AppColors.warning,
    };

    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: bg,
        border: Border.all(color: border),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 32, color: iconColor),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('KẾT QUẢ XÁC MINH',
                    style: TextStyle(fontSize: 11, letterSpacing: 1.5, fontWeight: FontWeight.w600, color: fg)),
                const SizedBox(height: 6),
                Text(title, style: TextStyle(fontSize: 21, fontWeight: FontWeight.w700, color: fg)),
                const SizedBox(height: 6),
                Text(detail, style: TextStyle(fontSize: 14, color: fg)),
                if (degree.revoked && degree.revokedAt != null) ...[
                  const SizedBox(height: 6),
                  Text('Ngày thu hồi: ${formatDate(degree.revokedAt)}', style: TextStyle(fontSize: 13, color: fg)),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _DegreeCard extends StatelessWidget {
  final DegreeVerification degree;

  const _DegreeCard({required this.degree});

  @override
  Widget build(BuildContext context) {
    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            (degree.certificateType.isEmpty ? 'Văn bằng' : degree.certificateType).toUpperCase(),
            style: const TextStyle(fontSize: 12, letterSpacing: 1.5, fontWeight: FontWeight.w600, color: AppColors.primary),
          ),
          const SizedBox(height: 6),
          Text(degree.fullName, style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w700)),
          const SizedBox(height: 4),
          Text(
            [degree.name, degree.universityName].where((s) => s.isNotEmpty).join(' · '),
            style: const TextStyle(color: AppColors.muted),
          ),
          const Divider(height: 28),
          Row(
            children: [
              Expanded(child: _Field(label: 'Số hiệu', value: degree.serialNumber.isEmpty ? '—' : degree.serialNumber)),
              Expanded(child: _Field(label: 'Ngày cấp', value: formatDate(degree.issueDate))),
            ],
          ),
        ],
      ),
    );
  }
}

class _Field extends StatelessWidget {
  final String label;
  final String value;

  const _Field({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: const TextStyle(fontSize: 12, color: AppColors.muted)),
        const SizedBox(height: 4),
        Text(value, style: const TextStyle(fontWeight: FontWeight.w600)),
      ],
    );
  }
}

enum _CheckState { pass, fail, pending }

class _ChecksGrid extends StatelessWidget {
  final VerificationResult result;

  const _ChecksGrid({required this.result});

  @override
  Widget build(BuildContext context) {
    final blockchain = result.blockchain;
    final checks = <(String, _CheckState, String)>[
      (
        'Chữ ký ${result.algorithmLabel}',
        result.signatureValid ? _CheckState.pass : _CheckState.fail,
        result.signatureValid ? 'Hợp lệ' : 'Chưa đạt',
      ),
      (
        'Toàn vẹn PDF',
        !result.fileIntegrityChecked
            ? _CheckState.pending
            : (result.fileIntegrityValid ? _CheckState.pass : _CheckState.fail),
        !result.fileIntegrityChecked
            ? 'Chưa kiểm tra được'
            : (result.fileIntegrityValid ? 'Khớp bản đã ký' : 'Không khớp'),
      ),
      (
        'Blockchain',
        blockchain.valid
            ? _CheckState.pass
            : (blockchain.status == 'not_anchored' ? _CheckState.pending : _CheckState.fail),
        blockchain.valid ? 'Đã xác nhận' : blockchain.label,
      ),
    ];

    return Column(
      children: [
        for (final (label, state, value) in checks) ...[
          _CheckTile(label: label, state: state, value: value),
          const SizedBox(height: 10),
        ],
        if (blockchain.batchId.isNotEmpty)
          _Panel(
            child: _Field(label: 'Mã lô trên Blockchain', value: blockchain.batchId),
          ),
      ],
    );
  }
}

class _CheckTile extends StatelessWidget {
  final String label;
  final _CheckState state;
  final String value;

  const _CheckTile({required this.label, required this.state, required this.value});

  @override
  Widget build(BuildContext context) {
    final (Color bg, Color border, Color fg, IconData icon) = switch (state) {
      _CheckState.pass => (const Color(0xFFECFDF5), const Color(0xFFA7F3D0), const Color(0xFF047857), Icons.check_circle),
      _CheckState.fail => (const Color(0xFFFFF1F2), const Color(0xFFFECDD3), const Color(0xFFBE123C), Icons.cancel),
      _CheckState.pending => (Colors.white, const Color(0xFFE2E8F0), const Color(0xFF475569), Icons.radio_button_unchecked),
    };
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: bg,
        border: Border.all(color: border),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          Icon(icon, color: fg, size: 22),
          const SizedBox(width: 12),
          Expanded(child: Text(label, style: const TextStyle(color: AppColors.muted))),
          Flexible(
            child: Text(value,
                textAlign: TextAlign.right, style: TextStyle(color: fg, fontWeight: FontWeight.w600)),
          ),
        ],
      ),
    );
  }
}

class _FileCompareCard extends StatefulWidget {
  final String expectedHash;

  const _FileCompareCard({required this.expectedHash});

  @override
  State<_FileCompareCard> createState() => _FileCompareCardState();
}

class _FileCompareCardState extends State<_FileCompareCard> {
  bool _checking = false;
  String? _fileName;
  String? _hash;
  String? _error;

  Future<void> _pickAndCompare() async {
    setState(() {
      _error = null;
      _hash = null;
    });
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['pdf'],
      withData: true,
    );
    final file = picked?.files.single;
    if (file == null) return;
    if (file.bytes == null) {
      setState(() => _error = 'Không đọc được tệp đã chọn.');
      return;
    }
    setState(() => _checking = true);
    // Hashed on the device; the file is never uploaded.
    final digest = sha256.convert(file.bytes!).toString();
    if (!mounted) return;
    setState(() {
      _checking = false;
      _fileName = file.name;
      _hash = digest;
    });
  }

  @override
  Widget build(BuildContext context) {
    final match = _hash != null && _hash == widget.expectedHash;
    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Row(
            children: [
              Icon(Icons.upload_file, size: 20),
              SizedBox(width: 8),
              Text('Đối chiếu tệp PDF của bạn', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
            ],
          ),
          const SizedBox(height: 6),
          const Text(
            'Chọn tệp PDF văn bằng bạn nhận được để kiểm tra. Tệp được băm ngay trên điện thoại, không tải lên máy chủ.',
            style: TextStyle(color: AppColors.muted, fontSize: 13),
          ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            onPressed: _checking || widget.expectedHash.isEmpty ? null : _pickAndCompare,
            icon: const Icon(Icons.picture_as_pdf_outlined),
            label: Text(_checking ? 'Đang tính mã băm…' : 'Chọn tệp PDF'),
          ),
          if (_error != null) ...[
            const SizedBox(height: 10),
            Text(_error!, style: const TextStyle(color: AppColors.danger)),
          ],
          if (_hash != null) ...[
            const SizedBox(height: 12),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: match ? const Color(0xFFECFDF5) : const Color(0xFFFFF1F2),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    match ? 'Tệp trùng khớp với bản được lưu' : 'Tệp không khớp — có thể đã bị thay đổi',
                    style: TextStyle(
                      fontWeight: FontWeight.w600,
                      color: match ? const Color(0xFF065F46) : const Color(0xFF9F1239),
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text('$_fileName · SHA-256: $_hash', style: const TextStyle(fontSize: 12)),
                  const SizedBox(height: 6),
                  const Text(
                    'Kết quả đối chiếu tệp không thay thế việc kiểm tra chữ ký và trạng thái thu hồi.',
                    style: TextStyle(fontSize: 12, color: AppColors.muted),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _Panel extends StatelessWidget {
  final Widget child;

  const _Panel({required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        border: Border.all(color: const Color(0xFFE2E8F0)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: child,
    );
  }
}
