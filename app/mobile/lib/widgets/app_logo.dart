import 'package:flutter/material.dart';

import '../theme.dart';

/// The PQC emblem used by the web app. Its centre is transparent and its ring
/// text is white, so it always sits on the brand navy disc.
class AppLogo extends StatelessWidget {
  final double size;

  const AppLogo({super.key, this.size = 40});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: AppColors.navy,
        shape: BoxShape.circle,
        boxShadow: [
          BoxShadow(
            color: const Color(0xFF01D0FD).withValues(alpha: 0.25),
            blurRadius: size * 0.25,
          ),
        ],
      ),
      padding: EdgeInsets.all(size * 0.04),
      child: Image.asset('assets/images/app_logo.png', fit: BoxFit.contain),
    );
  }
}
