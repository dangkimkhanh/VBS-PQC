import 'package:flutter/material.dart';

import '../services/api_service.dart';
import '../theme.dart';
import '../widgets/app_logo.dart';
import 'qr_scan_screen.dart';
import 'verification_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  final _codeController = TextEditingController();

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  void _verifyTypedCode() {
    FocusScope.of(context).unfocus();
    final id = ApiService.parseVerificationCode(_codeController.text);
    if (id == null) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
        content: Text('Mã xác minh không hợp lệ. Vui lòng kiểm tra lại mã in trên văn bằng hoặc quét mã QR.'),
      ));
      return;
    }
    Navigator.push(context, MaterialPageRoute(builder: (_) => VerificationScreen(degreeId: id)));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 24, 20, 24),
          children: [
            const Row(
              children: [
                AppLogo(size: 44),
                SizedBox(width: 10),
                Text('Văn bằng số PQC', style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700)),
              ],
            ),
            const SizedBox(height: 40),
            const Text(
              'Xác minh văn bằng số',
              style: TextStyle(fontSize: 28, fontWeight: FontWeight.w800, height: 1.2),
            ),
            const SizedBox(height: 10),
            const Text(
              'Kiểm tra chữ ký số hậu lượng tử ML-DSA, tính toàn vẹn của tệp PDF và bằng chứng trên Blockchain. '
              'Không cần tài khoản.',
              style: TextStyle(fontSize: 15, color: AppColors.muted, height: 1.4),
            ),
            const SizedBox(height: 32),
            SizedBox(
              height: 56,
              child: FilledButton.icon(
                onPressed: () => Navigator.push(
                  context,
                  MaterialPageRoute(builder: (_) => const QrScanScreen()),
                ),
                icon: const Icon(Icons.qr_code_scanner),
                label: const Text('Quét mã QR trên văn bằng', style: TextStyle(fontSize: 16)),
              ),
            ),
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 20),
              child: Row(
                children: [
                  Expanded(child: Divider()),
                  Padding(
                    padding: EdgeInsets.symmetric(horizontal: 12),
                    child: Text('hoặc', style: TextStyle(color: AppColors.muted)),
                  ),
                  Expanded(child: Divider()),
                ],
              ),
            ),
            TextField(
              controller: _codeController,
              textInputAction: TextInputAction.search,
              onSubmitted: (_) => _verifyTypedCode(),
              decoration: InputDecoration(
                labelText: 'Mã xác minh hoặc liên kết xác minh',
                hintText: 'Mã 24 ký tự in trên văn bằng',
                filled: true,
                fillColor: Colors.white,
                border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
              ),
            ),
            const SizedBox(height: 12),
            SizedBox(
              height: 52,
              child: OutlinedButton.icon(
                onPressed: _verifyTypedCode,
                icon: const Icon(Icons.search),
                label: const Text('Xác minh'),
              ),
            ),
            const SizedBox(height: 40),
            const _Feature(
              icon: Icons.enhanced_encryption_outlined,
              title: 'Chữ ký hậu lượng tử',
              text: 'Văn bằng được ký bằng ML-DSA (FIPS 204), an toàn trước máy tính lượng tử.',
            ),
            const _Feature(
              icon: Icons.picture_as_pdf_outlined,
              title: 'Đối chiếu tệp PDF',
              text: 'So khớp mã băm SHA-256 của tệp bạn nhận được ngay trên điện thoại.',
            ),
            const _Feature(
              icon: Icons.hub_outlined,
              title: 'Bằng chứng Blockchain',
              text: 'Lô văn bằng được ghi trên Hyperledger Fabric, không thể sửa đổi.',
            ),
            const SizedBox(height: 24),
            const _Footer(),
          ],
        ),
      ),
    );
  }
}

class _Footer extends StatelessWidget {
  const _Footer();

  @override
  Widget build(BuildContext context) {
    return const Column(
      children: [
        Divider(),
        SizedBox(height: 16),
        Text(
          'Tin cậy hôm nay · An toàn cho tương lai',
          style: TextStyle(color: AppColors.muted, fontSize: 13),
        ),
        SizedBox(height: 10),
        Text('Phiên bản 1.0.0', style: TextStyle(color: AppColors.muted, fontSize: 12)),
      ],
    );
  }
}

class _Feature extends StatelessWidget {
  final IconData icon;
  final String title;
  final String text;

  const _Feature({required this.icon, required this.title, required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: const Color(0xFFEEF2FF),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(icon, size: 20, color: AppColors.primary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: const TextStyle(fontWeight: FontWeight.w600)),
                const SizedBox(height: 2),
                Text(text, style: const TextStyle(color: AppColors.muted, fontSize: 13)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
