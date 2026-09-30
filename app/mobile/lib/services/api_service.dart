import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

import '../config.dart';
import '../models/degree_verification.dart';

class VerificationException implements Exception {
  final String message;
  const VerificationException(this.message);

  @override
  String toString() => message;
}

class ApiService {
  static final _idPattern = RegExp(r'^[a-fA-F0-9]{24}$');
  static final _linkPattern = RegExp(r'/verify/([^/?#]+)');

  /// Accepts the code printed on the diploma, or a verification link such as
  /// the one encoded in its QR code. Returns null when neither matches.
  static String? parseVerificationCode(String input) {
    var value = input.trim();
    final link = _linkPattern.firstMatch(value);
    if (link != null) value = Uri.decodeComponent(link.group(1)!);
    return _idPattern.hasMatch(value) ? value.toLowerCase() : null;
  }

  /// Runs the public verification (ML-DSA signature, PDF integrity and the
  /// blockchain proof) for one diploma.
  static Future<DegreeVerification> verifyDegree(String id) async {
    final uri = Uri.parse('$apiBaseUrl/api/v1/public/degrees/$id');
    final http.Response response;
    try {
      response = await http
          .get(uri, headers: const {'Accept': 'application/json'})
          .timeout(const Duration(seconds: 30));
    } on TimeoutException {
      throw const VerificationException('Máy chủ phản hồi quá lâu. Vui lòng thử lại.');
    } on SocketException {
      throw const VerificationException('Không kết nối được máy chủ. Kiểm tra kết nối mạng.');
    } on http.ClientException {
      throw const VerificationException('Không kết nối được máy chủ. Kiểm tra kết nối mạng.');
    }

    Map<String, dynamic> body = const {};
    try {
      body = (jsonDecode(utf8.decode(response.bodyBytes)) as Map).cast<String, dynamic>();
    } catch (_) {}

    if (response.statusCode == 200 && body['data'] is Map) {
      return DegreeVerification.fromJson((body['data'] as Map).cast<String, dynamic>());
    }
    final serverMessage = body['error'] is String ? body['error'] as String : '';
    switch (response.statusCode) {
      case 400:
        throw VerificationException(serverMessage.isNotEmpty ? serverMessage : 'Mã xác minh không hợp lệ.');
      case 404:
        throw const VerificationException('Không tìm thấy văn bằng đã cấp với mã này.');
      case 429:
        throw const VerificationException('Bạn xác minh quá nhiều lần. Vui lòng thử lại sau ít phút.');
      default:
        throw VerificationException(
          serverMessage.isNotEmpty ? serverMessage : 'Chưa thể xác minh. Vui lòng thử lại.',
        );
    }
  }
}
