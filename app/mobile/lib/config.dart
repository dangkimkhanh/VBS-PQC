/// Backend of the public verification service. Override at build time with
/// `flutter build apk --dart-define=API_BASE_URL=https://your-domain`.
const String apiBaseUrl = String.fromEnvironment(
  'API_BASE_URL',
  defaultValue: 'https://54.79.163.176.nip.io',
);
