'use client';

export default function PrivacyPolicy() {
  return (
    <main className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-950 dark:to-slate-900 px-4 py-16">
      <div className="max-w-4xl mx-auto">
        <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 p-8 md:p-12 shadow-lg">
          <h1 className="text-4xl font-black mb-2 text-slate-900 dark:text-white">Privacy Policy</h1>
          <p className="text-sm text-slate-500 mb-8">Last updated: June 2026 | For 1herosocial.ai</p>

          <div className="space-y-8 text-slate-700 dark:text-slate-300">
            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">1. Introduction</h2>
              <p>1herosocial.ai ("we", "us", "our", or "Company") operates the 1herosocial.ai website and application. This page informs you of our policies regarding the collection, use, and disclosure of personal data when you use our Service and the choices you have associated with that data.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">2. Information Collection and Use</h2>
              <p className="mb-3">We collect several different types of information for various purposes to provide and improve our Service to you.</p>

              <h3 className="text-lg font-semibold mb-2 text-slate-800 dark:text-slate-100">Types of Data Collected:</h3>
              <ul className="list-disc pl-6 space-y-2">
                <li><strong>Personal Data:</strong> Name, email address, phone number, studio/business information</li>
                <li><strong>Communication Data:</strong> WhatsApp, Facebook Messenger, Instagram messages and conversations</li>
                <li><strong>Payment Data:</strong> Transaction history, subscription information (processed securely by Stripe)</li>
                <li><strong>Usage Data:</strong> IP address, browser type, pages visited, time spent, feature usage</li>
                <li><strong>Device Data:</strong> Device type, operating system, browser information</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">3. Use of Data</h2>
              <p>1herosocial.ai uses the collected data for various purposes:</p>
              <ul className="list-disc pl-6 mt-2 space-y-1">
                <li>To provide, maintain, and improve our services</li>
                <li>To process transactions and send related information</li>
                <li>To send technical notices and support messages</li>
                <li>To respond to inquiries and provide customer support</li>
                <li>To monitor and analyze trends and usage of our services</li>
                <li>To detect, prevent, and address fraud and security issues</li>
                <li>To comply with legal obligations and enforce our agreements</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">4. Security of Data</h2>
              <p>The security of your data is important to us but remember that no method of transmission over the Internet or method of electronic storage is 100% secure. While we strive to use commercially acceptable means to protect your Personal Data, we cannot guarantee its absolute security.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">5. Data Retention</h2>
              <p>We retain your data for as long as your account is active or as needed to provide you services. You can request deletion of your data at any time through your account settings or by contacting us. Some data may be retained longer for:</p>
              <ul className="list-disc pl-6 mt-2 space-y-1">
                <li>Legal compliance and regulatory requirements</li>
                <li>Fraud prevention and security purposes</li>
                <li>Backup and archival purposes (maximum 1 year)</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">6. Your Rights and Choices</h2>
              <p>You have the following rights regarding your personal data:</p>
              <ul className="list-disc pl-6 mt-2 space-y-1">
                <li><strong>Right to Access:</strong> Request a copy of your personal data</li>
                <li><strong>Right to Rectification:</strong> Correct inaccurate or incomplete data</li>
                <li><strong>Right to Erasure:</strong> Request deletion of your account and associated data</li>
                <li><strong>Right to Opt-Out:</strong> Unsubscribe from communications</li>
                <li><strong>Right to Data Portability:</strong> Request your data in a portable format</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">7. Account Deletion</h2>
              <p>You can delete your account permanently at any time:</p>
              <ol className="list-decimal pl-6 mt-2 space-y-1">
                <li>Go to Settings → Security → Delete Account</li>
                <li>Enter your email address to confirm</li>
                <li>Click "Permanently Delete Account"</li>
                <li>Your account and all associated data will be deleted within 24 hours</li>
              </ol>
              <p className="mt-4 text-sm text-slate-600 dark:text-slate-400">
                <strong>Warning:</strong> This action is irreversible. All your data, conversations, and subscription information will be permanently deleted.
              </p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">8. Third-Party Services</h2>
              <p>We use trusted third-party services:</p>
              <ul className="list-disc pl-6 mt-2 space-y-1">
                <li><strong>Stripe:</strong> Payment processing (PCI DSS compliant)</li>
                <li><strong>Meta (WhatsApp, Messenger, Instagram):</strong> Messaging platforms</li>
                <li><strong>AWS S3:</strong> Secure file storage</li>
                <li><strong>Google Cloud:</strong> Infrastructure and backups</li>
              </ul>
              <p className="mt-2 text-sm">These services have their own privacy policies. We encourage you to review them.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">9. Changes to This Privacy Policy</h2>
              <p>We may update our Privacy Policy from time to time. We will notify you of any changes by updating the "Last updated" date of this Privacy Policy.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">10. Contact Us</h2>
              <p>If you have any questions about this Privacy Policy, please contact us:</p>
              <div className="mt-4 space-y-2">
                <p><strong>Email:</strong> govind.infaira@gmail.com</p>
                <p><strong>Website:</strong> https://1herosocial.ai</p>
              </div>
            </section>

            <div className="mt-8 p-4 bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg">
              <p className="text-sm text-blue-900 dark:text-blue-200">
                <strong>GDPR Compliance:</strong> For users in the EU, this privacy policy complies with GDPR requirements. You have additional rights to file complaints with your local data protection authority.
              </p>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}
