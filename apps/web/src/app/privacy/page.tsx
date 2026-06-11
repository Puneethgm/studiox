'use client';

export default function PrivacyPolicy() {
  return (
    <main className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-950 dark:to-slate-900 px-4 py-16">
      <div className="max-w-4xl mx-auto">
        <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 p-8 md:p-12 shadow-lg">
          <h1 className="text-4xl font-black mb-2 text-slate-900 dark:text-white">Privacy Policy</h1>
          <p className="text-sm text-slate-500 mb-1">How We Collect, Use &amp; Protect Your Data</p>
          <p className="text-sm text-slate-500 mb-8">Effective Date: June 9, 2026</p>

          <div className="space-y-8 text-slate-700 dark:text-slate-300">
            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">1. Introduction</h2>
              <p className="mb-3">1HeroSocial ("we", "us", "our") is committed to protecting the privacy of Studio clients, their staff, and their customers' leads. This Privacy Policy explains how we collect, use, disclose, and safeguard personal data when you use the 1HeroSocial Platform.</p>
              <p>By using the Platform, you consent to the data practices described in this Privacy Policy. If you are a Studio using our Platform, you acknowledge that you act as a data controller in respect of your customers' Lead data, and we act as your data processor.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">2. Data We Collect</h2>
              <div className="space-y-4">
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">2.1 Account &amp; Registration Data</h3>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Full name, email address, and hashed password of Studio Admins and staff.</li>
                    <li>Studio name, brand assets (logo, brand colour), and business identifiers.</li>
                    <li>Billing information (processed and stored by our payment processor; we do not store card details).</li>
                    <li>IP addresses and device information during login.</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">2.2 Lead &amp; Customer Data (on behalf of Studios)</h3>
                  <p className="mb-2">When prospective customers submit enquiry forms on Studio lead-capture pages, we collect on behalf of the Studio:</p>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Name, email address, phone number.</li>
                    <li>Campaign source and referral attribution data.</li>
                    <li>Conversation history via WhatsApp and other integrated channels.</li>
                    <li>Lead status and pipeline progression events.</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">2.3 Usage &amp; Operational Data</h3>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Platform activity logs (pages visited, features used, actions taken).</li>
                    <li>API request logs including request IDs and tenant identifiers.</li>
                    <li>Performance and error telemetry.</li>
                    <li>Integration data synced to Google Sheets and other connected services.</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">2.4 AI Processing Data</h3>
                  <p>When you use our AI Services, the content you provide (messages, campaign briefs, lead notes) is processed by our AI models to generate responses and recommendations. We do not use your Studio's or your customers' personal data to train our AI models without explicit consent.</p>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">3. How We Use Your Data</h2>
              <div className="space-y-4">
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">3.1 Platform Operation</h3>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Authenticating users and enforcing role-based access controls.</li>
                    <li>Provisioning and managing Studio accounts and multi-tenant isolation.</li>
                    <li>Delivering the Lead management pipeline, messaging inbox, and campaign tracking features.</li>
                    <li>Syncing data to connected integrations (Google Sheets, WhatsApp, Ads platforms).</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">3.2 AI &amp; Automation Features</h3>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Generating marketing content, reply drafts, and lead nurture sequences.</li>
                    <li>Scoring leads and recommending next-best actions.</li>
                    <li>Routing and attributing leads to campaigns and channels.</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">3.3 Communications</h3>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Sending transactional emails (account alerts, system notifications).</li>
                    <li>Sending product updates and feature announcements (you may opt out).</li>
                    <li>Responding to support requests.</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">3.4 Security &amp; Compliance</h3>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Detecting and preventing fraud, abuse, and unauthorized access.</li>
                    <li>Maintaining immutable audit logs of privileged administrative actions.</li>
                    <li>Complying with applicable legal obligations and responding to lawful requests.</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">3.5 Analytics &amp; Improvement</h3>
                  <ul className="list-disc pl-6 space-y-1">
                    <li>Aggregated, anonymized usage analytics to improve the Platform.</li>
                    <li>Performance monitoring and infrastructure optimization.</li>
                  </ul>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">4. Data Sharing &amp; Disclosure</h2>
              <p className="mb-4">We do not sell your personal data. We share data only in the following circumstances:</p>
              <div className="space-y-4">
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">4.1 Service Providers</h3>
                  <p>We engage trusted third-party processors to help operate the Platform, including cloud hosting (AWS), payment processing, email delivery, and analytics. These processors are contractually bound to handle data only as instructed by us.</p>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">4.2 Third-Party Integrations</h3>
                  <p>When you connect integrations (Google Sheets, Meta WhatsApp, Ads platforms), data is shared with those providers subject to your own configuration and their terms. You are responsible for ensuring lawful basis for such data transfers.</p>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">4.3 Legal Requirements</h3>
                  <p>We may disclose data if required by law, court order, or governmental authority, or to protect the rights, safety, or property of 1HeroSocial, our users, or the public.</p>
                </div>
                <div>
                  <h3 className="text-lg font-semibold mb-2 text-slate-900 dark:text-white">4.4 Business Transfers</h3>
                  <p>In the event of a merger, acquisition, or sale of assets, your data may be transferred as part of that transaction. We will notify you before your data becomes subject to a materially different privacy policy.</p>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">5. Data Retention</h2>
              <ul className="list-disc pl-6 space-y-2">
                <li>Account data is retained for the duration of your subscription plus 7 years for compliance purposes.</li>
                <li>Lead data is retained as long as your Studio account is active, and for 30 days after account termination to allow data export.</li>
                <li>Audit logs and activity records are retained for 7 years.</li>
                <li>AI processing logs are retained for 90 days and then anonymized.</li>
                <li>You may request deletion of personal data subject to legal retention obligations.</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">6. Data Security</h2>
              <p className="mb-3">We implement commercially reasonable technical and organizational measures to protect your data, including:</p>
              <ul className="list-disc pl-6 space-y-1">
                <li>AES-256 encryption for data in transit and at rest.</li>
                <li>HTTP-only cookies and JWT-based session management.</li>
                <li>Row-level security ensuring strict multi-tenant data isolation.</li>
                <li>Immutable audit logs for all privileged operations.</li>
                <li>Regular security assessments and penetration testing (roadmap).</li>
              </ul>
              <p className="mt-3">Despite these measures, no system is entirely secure. You are responsible for maintaining the security of your account credentials.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">7. Your Rights</h2>
              <p className="mb-3">Subject to applicable law, you have the following rights regarding your personal data:</p>
              <ul className="list-disc pl-6 space-y-2">
                <li><strong>Access:</strong> Request a copy of the personal data we hold about you.</li>
                <li><strong>Rectification:</strong> Request correction of inaccurate or incomplete data.</li>
                <li><strong>Erasure:</strong> Request deletion of your personal data (subject to legal retention requirements).</li>
                <li><strong>Portability:</strong> Request your data in a structured, machine-readable format.</li>
                <li><strong>Restriction:</strong> Request that we limit processing of your data in certain circumstances.</li>
                <li><strong>Objection:</strong> Object to processing based on legitimate interests or for direct marketing.</li>
              </ul>
              <p className="mt-3">To exercise any of these rights, contact us at <a href="mailto:1hersocialai@gmail.com" className="text-blue-600 dark:text-blue-400 underline">1hersocialai@gmail.com</a>. We will respond within 30 days.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">8. Account Deletion</h2>
              <p className="mb-3">You can delete your account permanently at any time:</p>
              <ol className="list-decimal pl-6 mb-4 space-y-1">
                <li>Go to Settings → Security → Delete Account</li>
                <li>Enter your email address to confirm</li>
                <li>Click "Permanently Delete Account"</li>
                <li>Your account and all associated data will be deleted within 24 hours</li>
              </ol>
              <p className="text-sm text-slate-600 dark:text-slate-400">
                <strong>Warning:</strong> This action is irreversible. All your data, conversations, and subscription information will be permanently deleted.
              </p>
              <p className="mt-3">For assistance with account deletion, contact <a href="mailto:1hersocialai@gmail.com" className="text-blue-600 dark:text-blue-400 underline">1hersocialai@gmail.com</a>.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">9. Cookies &amp; Tracking</h2>
              <p>The Platform uses HTTP-only cookies for session management and authentication. We do not use third-party advertising cookies on the admin platform. Public lead-capture forms may use minimal analytics cookies where required by Studio configuration. You can manage cookie preferences through your browser settings.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">10. Children's Privacy</h2>
              <p>The Platform is not directed at individuals under the age of 16. We do not knowingly collect personal data from children. If you believe we have inadvertently collected such data, contact us immediately and we will delete it.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">11. International Transfers</h2>
              <p>Our servers are currently hosted in AWS regions. If you are located outside of the hosting region, your data may be transferred internationally. We ensure appropriate safeguards are in place for such transfers in accordance with applicable data protection law.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">12. Changes to This Policy</h2>
              <p>We may update this Privacy Policy from time to time. We will notify you of material changes via email and by posting the updated policy on the Platform. Your continued use of the Platform following such notice constitutes acceptance of the updated policy.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">13. Contact Us</h2>
              <p className="mb-3">For any privacy-related questions, data subject requests, or concerns, please contact:</p>
              <div className="space-y-1">
                <p><strong>1HeroSocial — Data Privacy</strong></p>
                <p><strong>Email:</strong> <a href="mailto:1hersocialai@gmail.com" className="text-blue-600 dark:text-blue-400 underline">1hersocialai@gmail.com</a></p>
                <p><strong>Website:</strong> https://1herosocial.com</p>
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
