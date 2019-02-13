
namespace dotnetcore
{
    using System;
    using System.IO;
    using System.Security.Cryptography.X509Certificates;

    public class Program
    {
        public static void Main(string[] args)
        {
            // Input parsing.
            Console.WriteLine("Please enter the path to the KeyVault files or volume.");
            string folderPath = Console.ReadLine();

            Console.WriteLine("Please enter the secret name.");
            string secretName = Console.ReadLine();

            string fullPath = Path.Combine(folderPath, secretName);

            // Verifying secret file exists and path is OK.
            Console.WriteLine($"Reading secret at location: {fullPath}");
            if (!File.Exists(fullPath))
            {
                Console.WriteLine("File does not exist.");
                return;
            }          

            try
            {
                // Read all of the secret contents and convert to Byte array.
                string secretContents = File.ReadAllText(fullPath);
                Byte[] rawCertContent = Convert.FromBase64String(secretContents);

                // The constructor parses the Byte array into the certificate in memory.
                // If you need to enter a password, then there's an overloaded constructor with
                // the string password as the second parameter.
                X509Certificate2 certificate = new X509Certificate2(rawCertContent);

                Console.WriteLine("Successfully parsed the secret into a certificate");
                Console.WriteLine($"Subject name: {certificate.SubjectName.Name}");
                Console.WriteLine($"Thumbprint: {certificate.Thumbprint}");
            }
            catch (Exception e)
            {
                Console.WriteLine("Exception thrown while trying to read or parse the secret into a certificate.");
                Console.WriteLine($"Message: {e.Message}");
            }
        }
    }
}
