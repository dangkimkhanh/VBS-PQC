import 'package:flutter/material.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:image_picker/image_picker.dart';
import '../services/api_service.dart';
import 'verification_screen.dart';

class QrScanScreen extends StatefulWidget {
  const QrScanScreen({super.key});
  @override
  State<QrScanScreen> createState() => _QrScanScreenState();
}

class _QrScanScreenState extends State<QrScanScreen>
    with SingleTickerProviderStateMixin, WidgetsBindingObserver {
  late final MobileScannerController cameraController;
  late final AnimationController _animationController;

  bool hasPermission = false;
  bool isFlashOn = false;
  bool isDecoding = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);

    cameraController = MobileScannerController(
      detectionSpeed: DetectionSpeed.unrestricted,
      detectionTimeoutMs: 80,
      facing: CameraFacing.back,
      torchEnabled: false,
      formats: const [BarcodeFormat.qrCode],
      autoStart: false,
    );

    _animationController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 2),
    )..repeat(reverse: true);

    WidgetsBinding.instance.addPostFrameCallback((_) {
      _initCameraWithRetry();
    });
  }

  Future<void> _initCameraWithRetry() async {
    final status = await Permission.camera.request();
    if (!status.isGranted) return;

    setState(() => hasPermission = true);

    Future.delayed(const Duration(milliseconds: 100), () async {
      try {
        await cameraController.start();
      } catch (_) {
        await Future.delayed(const Duration(milliseconds: 200));
        try {
          await cameraController.start();
        } catch (e) {
          debugPrint("Lỗi mở camera");
        }
      }
    });
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) async {
    if (!hasPermission) return;
    if (state == AppLifecycleState.resumed) {
      await Future.delayed(const Duration(milliseconds: 200));
      try {
        await cameraController.start();
      } catch (_) {
        await Future.delayed(const Duration(milliseconds: 300));
        await cameraController.start();
      }
    } else if (state == AppLifecycleState.paused) {
      await cameraController.stop();
    }
  }

  @override
  void deactivate() {
    cameraController.stop();
    super.deactivate();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    cameraController.dispose();
    _animationController.dispose();
    super.dispose();
  }

  void _toggleFlash() async {
    setState(() => isFlashOn = !isFlashOn);
    try {
      await cameraController.toggleTorch();
    } catch (_) {
      setState(() => isFlashOn = !isFlashOn);
    }
  }

  Future<void> _pickFromGallery() async {
    final picker = ImagePicker();
    final XFile? image = await picker.pickImage(source: ImageSource.gallery);
    if (image == null) return;

    setState(() => isDecoding = true);
    try {
      final BarcodeCapture? cap =
      await cameraController.analyzeImage(image.path);
      if (cap != null && cap.barcodes.isNotEmpty) {
        final code = cap.barcodes.first.rawValue;
        if (code != null && code.isNotEmpty) {
          await _handleQrResult(code);
        } else {
          _showSnack("Không tìm thấy mã QR trong ảnh.");
        }
      } else {
        _showSnack("Không phát hiện mã QR trong ảnh.");
      }
    } catch (e) {
      _showSnack("Không đọc được mã QR từ ảnh.");
    } finally {
      if (mounted) setState(() => isDecoding = false);
    }
  }

  Future<void> _handleQrResult(String qrValue) async {
    // The QR code on a diploma holds its public verification link (…/verify/<id>).
    final id = ApiService.parseVerificationCode(qrValue);
    if (id == null) {
      _showSnack("Mã QR này không phải mã xác minh văn bằng.");
      await cameraController.start();
      return;
    }
    await cameraController.stop();
    if (!mounted) return;
    await Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => VerificationScreen(degreeId: id)),
    );
    await Future.delayed(const Duration(milliseconds: 150));
    if (mounted) await cameraController.start();
  }

  void _onDetect(BarcodeCapture capture) async {
    if (isDecoding) return;
    for (final barcode in capture.barcodes) {
      final code = barcode.rawValue;
      if (code != null && code.isNotEmpty) {
        setState(() => isDecoding = true);
        await cameraController.stop();
        try {
          await _handleQrResult(code);
        } finally {
          if (mounted) setState(() => isDecoding = false);
        }
        break;
      }
    }
  }

  void _showSnack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(msg),
        behavior: SnackBarBehavior.floating,
        margin: const EdgeInsets.all(16),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final size = MediaQuery.of(context).size;
    final scanBoxSize = size.width * 0.7;
    final center = Offset(size.width / 2, size.height / 2);

    if (!hasPermission) {
      return Scaffold(
        backgroundColor: Colors.black,
        body: Center(
          child: ElevatedButton(
            onPressed: _initCameraWithRetry,
            child: const Text("Cấp quyền camera"),
          ),
        ),
      );
    }

    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        children: [
          MobileScanner(controller: cameraController, onDetect: _onDetect),
          CustomPaint(
            size: Size(size.width, size.height),
            painter: _QrOverlayPainter(center: center, boxSize: scanBoxSize),
          ),
          Positioned(
            top: center.dy - scanBoxSize / 2 - 100,
            left: 0,
            right: 0,
            child: const Center(
              child: Text(
                'ĐẶT MÃ QR VÀO GIỮA KHUNG',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 18,
                    fontWeight: FontWeight.w600),
              ),
            ),
          ),
          AnimatedBuilder(
            animation: _animationController,
            builder: (context, _) {
              return Positioned(
                top: center.dy - scanBoxSize / 2 +
                    (scanBoxSize - 3) * _animationController.value,
                left: center.dx - scanBoxSize / 2 + 4,
                child: Container(
                  width: scanBoxSize - 8,
                  height: 3,
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      colors: [
                        Colors.greenAccent.withValues(alpha: 0.0),
                        Colors.greenAccent,
                        Colors.greenAccent.withValues(alpha: 0.0),
                      ],
                    ),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.greenAccent.withValues(alpha: 0.6),
                        blurRadius: 8,
                        spreadRadius: 2,
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
          Positioned(
            bottom: 100,
            left: 0,
            right: 0,
            child: Column(
              children: [
                if (isDecoding)
                  const Padding(
                    padding: EdgeInsets.only(bottom: 20),
                    child:
                    CircularProgressIndicator(color: Colors.greenAccent),
                  ),
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    _controlButton(
                      icon: isFlashOn ? Icons.flash_on : Icons.flash_off,
                      color: isFlashOn ? Colors.yellow : Colors.white,
                      onTap: _toggleFlash,
                    ),
                    _controlButton(
                      icon: Icons.photo_library_outlined,
                      color: Colors.white,
                      onTap: _pickFromGallery,
                    ),
                    _controlButton(
                      icon: Icons.close,
                      color: Colors.red,
                      onTap: () async {
                        await cameraController.stop();
                        if (context.mounted) Navigator.pop(context);
                      },
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _controlButton({
    required IconData icon,
    required Color color,
    required VoidCallback onTap,
  }) {
    return Container(
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: 0.2),
        shape: BoxShape.circle,
      ),
      child: IconButton(
        icon: Icon(icon, color: color, size: 28),
        onPressed: onTap,
      ),
    );
  }
}

class _QrOverlayPainter extends CustomPainter {
  final Offset center;
  final double boxSize;
  _QrOverlayPainter({required this.center, required this.boxSize});

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.black.withValues(alpha: 0.95)
      ..style = PaintingStyle.fill;

    final path = Path()..addRect(Rect.fromLTWH(0, 0, size.width, size.height));
    final rect =
    Rect.fromCenter(center: center, width: boxSize, height: boxSize);
    final transparent = Path()
      ..addRRect(RRect.fromRectAndRadius(rect, const Radius.circular(14)));

    path.addPath(transparent, Offset.zero);
    canvas.drawPath(
        Path.combine(PathOperation.difference, path, transparent), paint);

    final border = Paint()
      ..color = Colors.white.withValues(alpha: 0.95)
      ..strokeWidth = 2.5
      ..style = PaintingStyle.stroke;

    canvas.drawRRect(
        RRect.fromRectAndRadius(rect, const Radius.circular(14)), border);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
