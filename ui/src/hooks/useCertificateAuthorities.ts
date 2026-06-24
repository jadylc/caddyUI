import { useQuery } from "@tanstack/react-query";
import { getCertificateAuthorities, type CertificateAuthority } from "src/api/backend";

const fetchCertificateAuthorities = () => {
	return getCertificateAuthorities();
};

const useCertificateAuthorities = (options = {}) => {
	return useQuery<CertificateAuthority[], Error>({
		queryKey: ["certificate-authorities"],
		queryFn: fetchCertificateAuthorities,
		staleTime: 60 * 1000,
		...options,
	});
};

export { fetchCertificateAuthorities, useCertificateAuthorities };
