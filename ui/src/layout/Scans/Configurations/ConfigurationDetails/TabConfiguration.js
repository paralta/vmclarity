import React from 'react';
import { useNavigate } from 'react-router-dom';
import { useFetch } from 'hooks';
import TitleValueDisplay, { TitleValueDisplayColumn } from 'components/TitleValueDisplay';
import DoublePaneDisplay from 'components/DoublePaneDisplay';
import Button from 'components/Button';
import Title from 'components/Title';
import Loader from 'components/Loader';
import { ScopeDisplay, ScanTypesDisplay, InstancesDisplay } from 'layout/Scans/scopeDisplayUtils';
import { ROUTES, APIS } from 'utils/systemConsts';

const ConfigurationScansDisplay = ({configId}) => {
    const navigate = useNavigate();

    const [{loading, data, error}] = useFetch(APIS.SCANS, {queryParams: {"$filter": `scanConfig/id eq '${configId}'`, "$count": true}});
    
    if (error) {
        return null;
    }

    if (loading) {
        return <Loader />
    }
    
    return (
        <>
            <Title medium>Configuration's scans</Title>
            <Button onClick={() => navigate(ROUTES.SCANS)}>{`See all scans (${data?.count || 0})`}</Button>
        </>
    )
}

const TabConfiguration = ({data}) => {
    const {id, scope, scanFamiliesConfig} = data || {};
    const {all, regions, instanceTagSelector, instanceTagExclusion} = scope;
    
    return (
        <DoublePaneDisplay
            leftPaneDisplay={() => (
                <>
                    <Title medium>Configuration</Title>
                    <TitleValueDisplayColumn>
                        <TitleValueDisplay title="Scope"><ScopeDisplay all={all} regions={regions} /></TitleValueDisplay>
                        <TitleValueDisplay title="Included instances"><InstancesDisplay tags={instanceTagSelector}/></TitleValueDisplay>
                        <TitleValueDisplay title="Excluded instances"><InstancesDisplay tags={instanceTagExclusion}/></TitleValueDisplay>
                        <TitleValueDisplay title="Scan types"><ScanTypesDisplay scanFamiliesConfig={scanFamiliesConfig} /></TitleValueDisplay>
                    </TitleValueDisplayColumn>
                </>
            )}
            rightPlaneDisplay={() => (
                <ConfigurationScansDisplay configId={id} />
            )}
        />
    )
}

export default TabConfiguration;