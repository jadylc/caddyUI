import { useQuery } from "@tanstack/react-query";
import { type Certificate, type CertificateExpansion, getCertificates } from "src/api/backend";

const fetchCertificates = (expand?: CertificateExpansion[]) => {
	return getCertificates(expand);
};

const useCertificates = (expand?: CertificateExpansion[], options = {}) => {
	return useQuery<Certificate[], Error>({
		queryKey: ["certificates", { expand }],
		queryFn: () => fetchCertificates(expand),
		staleTime: 60 * 1000,
		refetchInterval: (query) =>
			query.state.data?.some((cert) => ["pending", "failed"].includes(cert.meta?.status)) ? 5000 : false,
		...options,
	});
};

export { fetchCertificates, useCertificates };
