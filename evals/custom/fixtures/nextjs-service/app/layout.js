// Eval fixture: minimal app-router root layout.
export const metadata = {
  title: 'nextjs-service',
  description: 'Eval fixture checkout service',
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
