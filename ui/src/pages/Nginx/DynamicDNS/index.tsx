import { HasPermission } from "src/components";
import { STREAMS, VIEW } from "src/modules/Permissions";
import TableWrapper from "./TableWrapper";

const DynamicDNS = () => {
	return (
		<HasPermission section={STREAMS} permission={VIEW} pageLoading loadingNoLogo>
			<TableWrapper />
		</HasPermission>
	);
};

export default DynamicDNS;
