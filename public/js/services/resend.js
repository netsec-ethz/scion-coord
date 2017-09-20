scionApp
    .factory('resendService', ["$http", "$q",'$httpParamSerializer', function($http, $q, $httpParamSerializer) {

    var resendService = {
        resendEmail: function(email){
           return $http({
                method: 'POST',
                url: '/api/resendLink',
                data: $httpParamSerializer({ 'email': email }),
                headers: { 'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8' }
            });
        }
    };
    return resendService;
}]);
