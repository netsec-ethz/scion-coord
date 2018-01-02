scionApp
    .factory('verificationService', ["$http", "$q",'$httpParamSerializer', function($http, $q, $httpParamSerializer) {

    return {
        verifyEmail: function(uuid){
           return $http({
                method: 'POST',
                url: '/api/verifyEmail',
                data: $httpParamSerializer({'uuid': uuid}),
                headers: {'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8'}
            });
        }
    };
}]);
